// Bind libc getpid() directly via extern("C") — no Zig shim, no runtime rebuild.
// Proves the whole chain: extern("C",...) -> real C symbol -> direct C-ABI call -> cint return.
extern("C", "getpid") function c_getpid(): cint;

function main(): i32 {
  let pid: i32 = c_getpid() as i32; // cint -> i32 explicit bridge
  if (pid > 0) {
    return 0;
  }
  return 1;
}
