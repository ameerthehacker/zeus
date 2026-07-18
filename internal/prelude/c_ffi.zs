// C-FFI primitives — the generic, library-agnostic marshalling layer for pure-Zeus C bindings.
// Each forwards directly (C ABI) to a zeus_* symbol in runtime/c_ffi_runtime.zig. The extern("zeus",
// "sym") form is sugar for a direct C-ABI binding to `zeus_sym` (the zeus_ prefix is added
// automatically) — same as extern("C", "zeus_sym"). Self-hosted prelude: registered as ambient
// primordial functions when the prelude loads (like the timer fns), so @std/fs, @std/os and the
// process global can call them without an import. The `c` name prefix marks them as low-level FFI
// primitives and avoids colliding with user code (e.g. a user function `free`).

// ---- string <-> C string ----
extern("zeus", "cstr_from_string") function cStrFromString(s: string): cstr;
extern("zeus", "string_from_cstr")  function cStrToString(p: cstr): string;
extern("zeus", "string_from_bytes") function cBytesToString(p: cptr, len: clong): string;
extern("zeus", "cstr_is_null")      function cIsNull(p: cptr): cint;

// ---- raw (non-GC) memory: stable buffers for C to fill; caller must cFree ----
extern("zeus", "malloc")  function cMalloc(size: csize): cptr;
extern("zeus", "realloc") function cRealloc(p: cptr, size: csize): cptr;
extern("zeus", "free")    function cFree(p: cptr): void;

// ---- u8[] buffer bridge: hand an existing Zeus byte array's data pointer to C ----
extern("zeus", "bytes_ptr") function cBytesPtr(bytes: u8[]): cptr;
extern("zeus", "bytes_len") function cBytesLen(bytes: u8[]): clong;

// ---- pointer read/write by byte offset (generic C-struct-field access) ----
extern("zeus", "ptr_read_i8")  function cReadI8(base: cptr, offset: clong): cint;
extern("zeus", "ptr_read_i16") function cReadI16(base: cptr, offset: clong): cint;
extern("zeus", "ptr_read_i32") function cReadI32(base: cptr, offset: clong): cint;
extern("zeus", "ptr_read_i64") function cReadI64(base: cptr, offset: clong): clong;
extern("zeus", "ptr_read_u32") function cReadU32(base: cptr, offset: clong): clong;
extern("zeus", "ptr_read_f64") function cReadF64(base: cptr, offset: clong): cdouble;
extern("zeus", "ptr_read_ptr") function cReadPtr(base: cptr, offset: clong): cptr;
extern("zeus", "ptr_offset")   function cPtrOffset(base: cptr, offset: clong): cptr;
extern("zeus", "ptr_write_i32") function cWriteI32(base: cptr, offset: clong, value: cint): void;
extern("zeus", "ptr_write_i64") function cWriteI64(base: cptr, offset: clong, value: clong): void;

// ---- errno + OS identity ----
extern("zeus", "errno")        function cErrno(): cint;
extern("zeus", "clear_errno")  function cClearErrno(): void;
extern("zeus", "os_platform")  function cOsPlatform(): cstr;
extern("zeus", "os_arch")      function cOsArch(): cstr;
extern("zeus", "os_totalmem")  function cOsTotalmem(): clong;
