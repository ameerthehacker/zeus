// @std/os — synchronous OS information, bound to libc + a few generic runtime helpers.
//
//   import { platform, arch, hostname, homedir, tmpdir, totalmem } from "@std/os";
//
// Mostly pure Zeus over libc: hostname/loadavg/uname are direct libc bindings (uname's struct is read
// field-by-field with per-target offsets, like fs.zs reads struct stat); homedir/tmpdir are env
// lookups via the ambient `process` global. Only platform/arch/totalmem/freemem/cpu-count remain
// runtime helpers, where the macOS/Linux split (sysctl vs sysconf) is cleaner than through the FFI.

const HOSTNAME_BUF_SIZE: csize = 256 as csize;
const ENV_HOME: string = "HOME";
const ENV_TMPDIR: string = "TMPDIR";
const DEFAULT_TMPDIR: string = "/tmp";
const LOADAVG_BUF_SIZE: csize = 24 as csize; // 3 x f64
const PLATFORM_LINUX: string = "linux";

// struct utsname is read field-by-field (like fs.zs reads struct stat): fields are 256 bytes each on
// macOS, 65 on Linux, so the release/version/machine offsets are per-target. sysname is at 0 on both.
const UTSNAME_BUF: csize = 1536 as csize; // macOS is 5 * 256 = 1280; over-allocate
const UTS_RELEASE_OFF_DARWIN: clong = 512 as clong;
const UTS_VERSION_OFF_DARWIN: clong = 768 as clong;
const UTS_MACHINE_OFF_DARWIN: clong = 1024 as clong;
const UTS_RELEASE_OFF_LINUX: clong = 130 as clong;
const UTS_VERSION_OFF_LINUX: clong = 195 as clong;
const UTS_MACHINE_OFF_LINUX: clong = 260 as clong;

// ---- raw libc bindings ----
@extern("C", "gethostname") function c_gethostname(buf: cptr, len: csize): cint;
@extern("C", "getloadavg")  function c_getloadavg(loadavg: cptr, nelem: cint): cint;
@extern("C", "uname")       function c_uname(buf: cptr): cint;

// ---- generic runtime helpers: freemem (sysctl null-ptr + sysconf split) and cpu count are cleaner
// resolved in the platform-aware runtime layer than through the FFI, so they stay runtime-backed.
@extern("zeus", "os_freemem")   function cOsFreemem(): clong;
@extern("zeus", "os_cpu_count") function cOsCpuCount(): clong;

// unameField calls uname once and reads the null-terminated string at the per-target offset.
function unameField(darwinOff: clong, linuxOff: clong): string {
  let buf: cptr = cMalloc(UTSNAME_BUF);
  if ((c_uname(buf) as i32) != 0) {
    cFree(buf);
    return "";
  }
  let off: clong = darwinOff;
  if (cStrToString(cOsPlatform()).equals(PLATFORM_LINUX)) {
    off = linuxOff;
  }
  let s: string = cStrToString(cPtrOffset(buf, off) as cstr);
  cFree(buf);
  return s;
}

// platform returns a Node-style OS string ("darwin", "linux").
export function platform(): string {
  return cStrToString(cOsPlatform());
}

// arch returns a Node-style architecture string ("arm64", "x64").
export function arch(): string {
  return cStrToString(cOsArch());
}

// hostname returns the system hostname.
export function hostname(): string {
  let buf: cptr = cMalloc(HOSTNAME_BUF_SIZE);
  let rc: i32 = c_gethostname(buf, HOSTNAME_BUF_SIZE) as i32;
  if (rc != 0) {
    cFree(buf);
    return "";
  }
  let name: string = cStrToString(buf as cstr);
  cFree(buf);
  return name;
}

// homedir returns the current user's home directory (from $HOME).
export function homedir(): string {
  return process.getEnv(ENV_HOME);
}

// tmpdir returns the directory for temporary files ($TMPDIR, else "/tmp").
export function tmpdir(): string {
  let t: string = process.getEnv(ENV_TMPDIR);
  if (t.length > 0) {
    return t;
  }
  return DEFAULT_TMPDIR;
}

// totalmem returns total physical memory in bytes.
export function totalmem(): i64 {
  return cOsTotalmem() as i64;
}

// freemem returns a best-effort snapshot of free physical memory in bytes.
export function freemem(): i64 {
  return cOsFreemem() as i64;
}

// availableParallelism returns the number of logical CPUs (>= 1), like Node's availableParallelism.
export function availableParallelism(): i32 {
  return cOsCpuCount() as i32;
}

// type returns the OS name from uname ("Darwin", "Linux"), matching Node's os.type().
export function type(): string {
  return unameField(0 as clong, 0 as clong); // sysname is at offset 0 on both targets
}

// release returns the kernel release string (uname -r).
export function release(): string {
  return unameField(UTS_RELEASE_OFF_DARWIN, UTS_RELEASE_OFF_LINUX);
}

// version returns the kernel version string (uname -v).
export function version(): string {
  return unameField(UTS_VERSION_OFF_DARWIN, UTS_VERSION_OFF_LINUX);
}

// machine returns the hardware name from uname ("arm64", "x86_64").
export function machine(): string {
  return unameField(UTS_MACHINE_OFF_DARWIN, UTS_MACHINE_OFF_LINUX);
}

// endianness returns the CPU byte order. Zeus targets (arm64, x64) are little-endian.
export function endianness(): string {
  return "LE";
}

// EOL is the platform line terminator ("\n" on the POSIX targets Zeus supports).
export function EOL(): string {
  return "\n";
}

// LoadAvg holds the 1/5/15-minute system load averages.
export class LoadAvg {
  public one: f64;
  public five: f64;
  public fifteen: f64;

  public constructor(one: f64, five: f64, fifteen: f64) {
    this.one = one;
    this.five = five;
    this.fifteen = fifteen;
  }
}

// loadavg returns the 1/5/15-minute load averages via getloadavg(3). Values are 0 where unavailable.
export function loadavg(): LoadAvg {
  let buf: cptr = cMalloc(LOADAVG_BUF_SIZE);
  let n: i32 = c_getloadavg(buf, 3 as cint) as i32;
  let one: f64 = 0.0;
  let five: f64 = 0.0;
  let fifteen: f64 = 0.0;
  if (n >= 1) {
    one = cReadF64(buf, 0 as clong) as f64;
  }
  if (n >= 2) {
    five = cReadF64(buf, 8 as clong) as f64;
  }
  if (n >= 3) {
    fifteen = cReadF64(buf, 16 as clong) as f64;
  }
  cFree(buf);
  return new LoadAvg(one, five, fifteen);
}
