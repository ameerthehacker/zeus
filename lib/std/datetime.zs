// @std/datetime — a Date type, a subset of JavaScript's Date, bound directly to libc time.
//
//   import { Date, now } from "@std/datetime";
//   let d: Date = new Date(now());
//   console.log(d.toISOString());
//
// Pure Zeus over libc (clock_gettime/localtime_r/gmtime_r) plus the ambient C-FFI primitives — no
// dedicated runtime helpers. The `struct tm` integer fields (tm_sec..tm_isdst) share one layout on
// macOS and Linux, so their byte offsets are constant; tm_gmtoff follows at offset 40 on both.
//
// Zeus has no overloaded constructors, so a Date is built from epoch milliseconds: `new Date(ms)`.
// Use the free `now()` for the current time (Zeus cannot yet resolve a static method across module
// boundaries, so `Date.now()` is exposed as `now()`). getX() accessors are local-time (like Node);
// toISOString() is UTC.

@extern("C", "clock_gettime") function c_clock_gettime(clk: cint, tp: cptr): cint;
@extern("C", "localtime_r")   function c_localtime_r(timep: cptr, result: cptr): cptr;
@extern("C", "gmtime_r")      function c_gmtime_r(timep: cptr, result: cptr): cptr;

const CLOCK_REALTIME: cint = 0 as cint;  // same value on macOS + Linux
const TIMESPEC_BUF: csize = 16 as csize; // { time_t tv_sec; long tv_nsec }
const TIME_T_BUF: csize = 8 as csize;
const TM_BUF: csize = 64 as csize;       // struct tm is 56 bytes; over-allocate

// struct tm integer field offsets (identical on macOS + Linux):
const TM_SEC: clong = 0 as clong;
const TM_MIN: clong = 4 as clong;
const TM_HOUR: clong = 8 as clong;
const TM_MDAY: clong = 12 as clong;
const TM_MON: clong = 16 as clong;
const TM_YEAR: clong = 20 as clong;   // years since 1900
const TM_WDAY: clong = 24 as clong;
const TM_GMTOFF: clong = 40 as clong; // long: seconds east of UTC

// now returns the current time in milliseconds since the Unix epoch (JavaScript's Date.now()).
export function now(): i64 {
  let ts: cptr = cMalloc(TIMESPEC_BUF);
  c_clock_gettime(CLOCK_REALTIME, ts);
  let sec: i64 = cReadI64(ts, 0 as clong) as i64;
  let nsec: i64 = cReadI64(ts, 8 as clong) as i64;
  cFree(ts);
  return sec * 1000 + nsec / 1000000;
}

// Zero-pad a non-negative integer to `width` digits.
function pad(n: i32, width: i32): string {
  return ("" + n).padStart(width, "0");
}

export class Date {
  private ms: i64;

  public constructor(ms: i64) {
    this.ms = ms;
  }

  // fillTm populates a caller-owned `struct tm` buffer for this Date's time (local or UTC). The
  // caller frees the returned buffer. Seconds are floored so pre-1970 (negative) millis are correct.
  private fillTm(local: boolean): cptr {
    let secs: i64 = this.ms / 1000;
    if (this.ms % 1000 < 0) {
      secs = secs - 1;
    }
    let tbuf: cptr = cMalloc(TIME_T_BUF);
    cWriteI64(tbuf, 0 as clong, secs as clong);
    let tm: cptr = cMalloc(TM_BUF);
    if (local) {
      c_localtime_r(tbuf, tm);
    } else {
      c_gmtime_r(tbuf, tm);
    }
    cFree(tbuf);
    return tm;
  }

  // field reads one int field of the broken-down time (local or UTC) at `offset`.
  private field(local: boolean, offset: clong): i32 {
    let tm: cptr = this.fillTm(local);
    let v: i32 = cReadI32(tm, offset) as i32;
    cFree(tm);
    return v;
  }

  public getTime(): i64 {
    return this.ms;
  }

  public getFullYear(): i32 {
    return this.field(true, TM_YEAR) + 1900;
  }

  // getMonth is 0-based (January = 0), matching JavaScript.
  public getMonth(): i32 {
    return this.field(true, TM_MON);
  }

  public getDate(): i32 {
    return this.field(true, TM_MDAY);
  }

  public getHours(): i32 {
    return this.field(true, TM_HOUR);
  }

  public getMinutes(): i32 {
    return this.field(true, TM_MIN);
  }

  public getSeconds(): i32 {
    return this.field(true, TM_SEC);
  }

  // getDay is the day of the week (Sunday = 0).
  public getDay(): i32 {
    return this.field(true, TM_WDAY);
  }

  public getMilliseconds(): i32 {
    let r: i64 = this.ms % 1000;
    if (r < 0) {
      r = r + 1000;
    }
    return r as i32;
  }

  // getTimezoneOffset returns minutes, positive west of UTC (JavaScript's convention).
  public getTimezoneOffset(): i32 {
    let tm: cptr = this.fillTm(true);
    let gmtoff: i64 = cReadI64(tm, TM_GMTOFF) as i64;
    cFree(tm);
    return (-(gmtoff / 60)) as i32;
  }

  // toISOString renders the UTC time as YYYY-MM-DDTHH:MM:SS.sssZ.
  public toISOString(): string {
    let y: i32 = this.field(false, TM_YEAR) + 1900;
    let mo: i32 = this.field(false, TM_MON) + 1;
    let d: i32 = this.field(false, TM_MDAY);
    let h: i32 = this.field(false, TM_HOUR);
    let mi: i32 = this.field(false, TM_MIN);
    let s: i32 = this.field(false, TM_SEC);
    return pad(y, 4) + "-" + pad(mo, 2) + "-" + pad(d, 2) + "T" +
      pad(h, 2) + ":" + pad(mi, 2) + ":" + pad(s, 2) + "." + pad(this.getMilliseconds(), 3) + "Z";
  }
}
