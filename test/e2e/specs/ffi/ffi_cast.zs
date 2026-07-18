// Bind libc abs() and round-trip the cint<->i32 numeric bridge in both directions.
extern("C", "abs") function c_abs(x: cint): cint;

function main(): i32 {
  let n: cint = -42 as cint;     // i32 -> cint
  let r: i32 = c_abs(n) as i32;  // cint arg, cint -> i32 result
  if (r == 42) {
    return 0;
  }
  return 1;
}
