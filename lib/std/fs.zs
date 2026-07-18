// @std/fs — synchronous file I/O, bound directly to libc/POSIX.
//
//   import { readFileSync, writeFileSync, existsSync } from "@std/fs";
//
// Pure Zeus over raw libc externs plus the ambient C-FFI primitives. Reads use open(O_RDONLY) (no
// mode, so no varargs) and writes use non-variadic creat(), avoiding both the variadic-open ABI
// gotcha and platform-specific O_* flag values.

// ---- open/access flags & creation modes (same values on macOS + Linux) ----
const O_RDONLY: cint = 0 as cint; // read-only open
const F_OK: cint = 0 as cint;     // access(): existence check
const FILE_MODE: cint = 420 as cint; // 0644, for creat()
const DIR_MODE: cint = 493 as cint;  // 0755, for mkdir()

// ---- read/write loop tuning ----
const READ_CHUNK_SIZE: i64 = 65536;      // initial read buffer / growth step
const MAX_FILE_BYTES: i64 = 2147483647;  // i32 max — Zeus string/array length limit

// ---- struct stat field byte offsets (validated via offsetof) ----
// macOS arm64:
const STAT_SIZE_OFFSET_DARWIN: clong = 96 as clong;
const STAT_MODE_OFFSET_DARWIN: clong = 4 as clong;
const STAT_MTIME_OFFSET_DARWIN: clong = 48 as clong;
// Linux glibc (st_size and st_mtim.tv_sec are identical across arches; only st_mode differs):
const STAT_SIZE_OFFSET_LINUX: clong = 48 as clong;
const STAT_MTIME_OFFSET_LINUX: clong = 88 as clong;
const STAT_MODE_OFFSET_LINUX_ARM64: clong = 16 as clong;
const STAT_MODE_OFFSET_LINUX_X64: clong = 24 as clong;
const STAT_BUF_SIZE: csize = 256 as csize; // struct stat is 144B on macOS; over-allocate

// ---- st_mode type bits (S_IF*, same octal on macOS + Linux) ----
const S_IFMT: i32 = 61440;  // 0xF000 — file-type mask
const S_IFDIR: i32 = 16384; // 0x4000 — directory
const S_IFREG: i32 = 32768; // 0x8000 — regular file

// ---- struct dirent d_name byte offset ----
const DIRENT_NAME_OFFSET_DARWIN: clong = 21 as clong;
const DIRENT_NAME_OFFSET_LINUX: clong = 19 as clong;

// ---- platform/arch identity (from @std C-FFI os helpers) ----
const PLATFORM_LINUX: string = "linux";
const ARCH_ARM64: string = "arm64";

// ---- raw libc/POSIX bindings (direct C ABI) ----
extern("C", "open")   function c_open(path: cstr, flags: cint): cint;   // used only with O_RDONLY
extern("C", "creat")  function c_creat(path: cstr, mode: cint): cint;   // O_WRONLY|O_CREAT|O_TRUNC
extern("C", "read")   function c_read(fd: cint, buf: cptr, count: csize): clong;
extern("C", "write")  function c_write(fd: cint, buf: cptr, count: csize): clong;
extern("C", "close")  function c_close(fd: cint): cint;
extern("C", "access") function c_access(path: cstr, mode: cint): cint;
extern("C", "mkdir")  function c_mkdir(path: cstr, mode: cint): cint;
extern("C", "unlink") function c_unlink(path: cstr): cint;
extern("C", "rename") function c_rename(oldPath: cstr, newPath: cstr): cint;
extern("C", "stat")     function c_stat(path: cstr, buf: cptr): cint;
extern("C", "opendir")  function c_opendir(path: cstr): cptr;
extern("C", "readdir")  function c_readdir(dir: cptr): cptr;
extern("C", "closedir") function c_closedir(dir: cptr): cint;

// readFileSync reads an entire file and returns its contents as a string. Throws on open/read error.
// A grow-and-read loop (rather than lseek-sizing + one read) correctly handles short reads, pipes
// and /proc files, and rejects files larger than a Zeus string can hold.
export function readFileSync(path: string): string {
  let fd: i32 = c_open(cStrFromString(path), O_RDONLY) as i32;
  if (fd < 0) {
    throw new Error("FileError", "cannot open file: " + path);
  }

  let cap: i64 = READ_CHUNK_SIZE;
  let buf: cptr = cMalloc(cap as csize);
  let total: i64 = 0;
  let failed: boolean = false;
  let tooLarge: boolean = false;
  let atEof: boolean = false;

  while (!atEof && !failed) {
    if (total >= cap) {
      let newCap: i64 = cap * 2;
      if (newCap > MAX_FILE_BYTES) {
        tooLarge = true;
        failed = true;
      } else {
        cap = newCap;
        buf = cRealloc(buf, cap as csize);
      }
    }
    if (!failed) {
      let want: i64 = cap - total;
      let n: i64 = c_read(fd as cint, cPtrOffset(buf, total as clong), want as csize) as i64;
      if (n < 0) {
        failed = true;
      } else if (n == 0) {
        atEof = true;
      } else {
        total = total + n;
      }
    }
  }

  c_close(fd as cint);
  if (failed) {
    cFree(buf);
    if (tooLarge) {
      throw new Error("FileError", "file too large: " + path);
    }
    throw new Error("FileError", "cannot read file: " + path);
  }

  let content: string = cBytesToString(buf, total as clong);
  cFree(buf);
  return content;
}

// writeFileSync writes a string to a file, creating or truncating it (mode 0644). It loops over the
// write to handle short writes and throws on any error.
export function writeFileSync(path: string, data: string): void {
  let fd: i32 = c_creat(cStrFromString(path), FILE_MODE) as i32;
  if (fd < 0) {
    throw new Error("FileError", "cannot create file: " + path);
  }

  let cbuf: cstr = cStrFromString(data);
  let len: i64 = data.length as i64;
  let written: i64 = 0;
  let failed: boolean = false;

  while (written < len && !failed) {
    let n: i64 = c_write(fd as cint, cPtrOffset(cbuf as cptr, written as clong), (len - written) as csize) as i64;
    if (n <= 0) {
      failed = true;
    } else {
      written = written + n;
    }
  }

  c_close(fd as cint);
  if (failed) {
    throw new Error("FileError", "cannot write file: " + path);
  }
}

// existsSync reports whether a path exists.
export function existsSync(path: string): boolean {
  return (c_access(cStrFromString(path), F_OK) as i32) == 0;
}

// mkdirSync creates a directory (mode 0755). Throws on failure.
export function mkdirSync(path: string): void {
  if ((c_mkdir(cStrFromString(path), DIR_MODE) as i32) != 0) {
    throw new Error("FileError", "cannot mkdir: " + path);
  }
}

// unlinkSync removes a file. Throws on failure.
export function unlinkSync(path: string): void {
  if ((c_unlink(cStrFromString(path)) as i32) != 0) {
    throw new Error("FileError", "cannot unlink: " + path);
  }
}

// renameSync renames/moves a file. Throws on failure.
export function renameSync(oldPath: string, newPath: string): void {
  if ((c_rename(cStrFromString(oldPath), cStrFromString(newPath)) as i32) != 0) {
    throw new Error("FileError", "cannot rename: " + oldPath);
  }
}

// Stats describes a filesystem entry (a subset of Node's fs.Stats).
export class Stats {
  public size: i64;
  public mtime: i64;
  private modeBits: i32;

  public constructor(size: i64, mtime: i64, modeBits: i32) {
    this.size = size;
    this.mtime = mtime;
    this.modeBits = modeBits;
  }

  public isDirectory(): boolean {
    return (this.modeBits & S_IFMT) == S_IFDIR;
  }

  public isFile(): boolean {
    return (this.modeBits & S_IFMT) == S_IFREG;
  }
}

// statSync returns metadata for a path. The C `struct stat` is read field-by-field using per-target
// byte offsets. The low 16 bits of st_mode carry the S_IF* type bits on both macOS and Linux, so a
// 16-bit read at the mode offset is portable.
export function statSync(path: string): Stats {
  let buf: cptr = cMalloc(STAT_BUF_SIZE);
  let rc: i32 = c_stat(cStrFromString(path), buf) as i32;
  if (rc != 0) {
    cFree(buf);
    throw new Error("FileError", "cannot stat: " + path);
  }

  let sizeOff: clong = STAT_SIZE_OFFSET_DARWIN;
  let modeOff: clong = STAT_MODE_OFFSET_DARWIN;
  let mtimeOff: clong = STAT_MTIME_OFFSET_DARWIN;
  if (cStrToString(cOsPlatform()).equals(PLATFORM_LINUX)) {
    sizeOff = STAT_SIZE_OFFSET_LINUX;
    mtimeOff = STAT_MTIME_OFFSET_LINUX;
    if (cStrToString(cOsArch()).equals(ARCH_ARM64)) {
      modeOff = STAT_MODE_OFFSET_LINUX_ARM64;
    } else {
      modeOff = STAT_MODE_OFFSET_LINUX_X64;
    }
  }

  let size: i64 = cReadI64(buf, sizeOff) as i64;
  let mtime: i64 = cReadI64(buf, mtimeOff) as i64;
  let modeBits: i32 = cReadI16(buf, modeOff) as i32;
  cFree(buf);
  return new Stats(size, mtime, modeBits);
}

// readdirSync returns the entries of a directory (excluding "." and ".."). The char[] d_name is an
// inline field of struct dirent read at a per-target offset. errno is cleared first so a readdir
// error (which also returns NULL) is not mistaken for a clean end-of-directory.
export function readdirSync(path: string): string[] {
  let names: string[] = new string[];
  let dir: cptr = c_opendir(cStrFromString(path));
  if (cIsNull(dir) as i32 != 0) {
    throw new Error("FileError", "cannot open directory: " + path);
  }

  let nameOff: clong = DIRENT_NAME_OFFSET_DARWIN;
  if (cStrToString(cOsPlatform()).equals(PLATFORM_LINUX)) {
    nameOff = DIRENT_NAME_OFFSET_LINUX;
  }

  cClearErrno();
  let entry: cptr = c_readdir(dir);
  while (cIsNull(entry) as i32 == 0) {
    let name: string = cStrToString(cPtrOffset(entry, nameOff) as cstr);
    if (!name.equals(".") && !name.equals("..")) {
      names.push(name);
    }
    entry = c_readdir(dir);
  }
  let err: i32 = cErrno() as i32;
  c_closedir(dir);
  if (err != 0) {
    throw new Error("FileError", "error reading directory: " + path);
  }
  return names;
}
