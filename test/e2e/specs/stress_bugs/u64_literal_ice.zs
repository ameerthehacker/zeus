// BUG (wiki/compiler-bugs.md "Unsigned literal >= 2^63 crashes the compiler"):
// codegen converts the constant with signed strconv.ParseInt (llvm_type.go), which
// overflows for u64 literals >= 2^63, panicking with "internal compiler error".
// When fixed (ParseUint for unsigned), this compiles and runs to exit 0.
function main(): i32 {
  let a: u64 = 9223372036854775808;
  if (a > 0) { return 0; }
  return 1;
}
