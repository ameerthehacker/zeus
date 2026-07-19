import { Date, now } from "@std/datetime";

// UTC assertions only (toISOString/getTime/getMilliseconds) so the test is timezone-independent.
// Distinct sentinels; 0 = all pass. Also prints the ISO string for a stdout check.
function main(): i32 {
  let ms: i64 = 1609459200123 as i64;   // 2021-01-01T00:00:00.123Z
  let d: Date = new Date(ms);

  console.log(d.toISOString());

  if (d.getTime() != ms) { return 1; }
  if (!d.toISOString().equals("2021-01-01T00:00:00.123Z")) { return 2; }
  if (d.getMilliseconds() != 123) { return 3; }

  // getTimezoneOffset is minutes in a sane range.
  let tz: i32 = d.getTimezoneOffset();
  if (tz < -900 || tz > 900) { return 4; }

  // now() should be well after 2021.
  if (now() < ms) { return 5; }

  // epoch
  let e: Date = new Date(0 as i64);
  if (!e.toISOString().equals("1970-01-01T00:00:00.000Z")) { return 6; }

  return 0;
}
