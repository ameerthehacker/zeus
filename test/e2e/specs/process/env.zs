// `process` is an ambient global object (like `console`) — no import needed.
function main(): i32 {
  process.setEnv("ZEUS_FFI_TEST", "hello");
  if (!process.hasEnv("ZEUS_FFI_TEST")) { return 1; }

  let v: string = process.getEnv("ZEUS_FFI_TEST");
  if (!v.equals("hello")) { return 2; }

  // An unset variable reads back as the empty string.
  let missing: string = process.getEnv("ZEUS_DEFINITELY_NOT_SET_12345");
  if (missing.length != 0) { return 3; }
  if (process.hasEnv("ZEUS_DEFINITELY_NOT_SET_12345")) { return 4; }

  return 0;
}
