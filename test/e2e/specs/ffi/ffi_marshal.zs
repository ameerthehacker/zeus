// Exercise the generic C-FFI marshalling layer (runtime/c_ffi_runtime.zig via the c_ffi prelude):
// raw malloc/write/read/free and a string -> cstr -> string round-trip.
function main(): i32 {
  // raw pointer write/read round-trip through a malloc'd buffer
  let buf: cptr = cMalloc(16 as csize);
  cWriteI32(buf, 0 as clong, 12345 as cint);
  cWriteI64(buf, 8 as clong, 9876543210 as clong);
  let a: i32 = cReadI32(buf, 0 as clong) as i32;
  let b: i64 = cReadI64(buf, 8 as clong) as i64;
  cFree(buf);
  if (a != 12345) { return 1; }
  if (b != 9876543210) { return 2; }

  // string -> C string -> string round-trip
  let original: string = "hello ffi";
  let cs: cstr = cStrFromString(original);
  let back: string = cStrToString(cs);
  if (!back.equals(original)) { return 3; }

  return 0;
}
