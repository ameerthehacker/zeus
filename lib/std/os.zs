// @std/os — synchronous OS information, bound to libc + a few generic runtime helpers.
//
//   import { platform, arch, hostname, homedir, tmpdir, totalmem } from "@std/os";
//
// platform/arch/totalmem come from small runtime helpers (avoiding struct utsname parsing and the
// macOS/Linux split); hostname/homedir/tmpdir are pure libc/env. Environment lookups use the ambient
// global `process` object.

const HOSTNAME_BUF_SIZE: csize = 256 as csize;
const ENV_HOME: string = "HOME";
const ENV_TMPDIR: string = "TMPDIR";
const DEFAULT_TMPDIR: string = "/tmp";

// ---- raw libc bindings ----
extern("C", "gethostname") function c_gethostname(buf: cptr, len: csize): cint;

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
