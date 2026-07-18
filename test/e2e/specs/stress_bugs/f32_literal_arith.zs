// BUG (wiki/compiler-bugs.md "f32 arithmetic with a float literal is rejected"):
// an f32 combined with an f64 float literal promotes to f64 and won't store back to f32.
// When fixed (literal adopts f32 in f32 context), this compiles and returns 0.
function main(): i32 {
  let a: f32 = 0.1;
  let b: f32 = a + 0.2;
  if (b > 0.0) { return 0; }
  return 1;
}
