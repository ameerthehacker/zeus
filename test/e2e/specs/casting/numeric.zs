// Unchecked numeric `as` casts: truncate toward zero, wrap on narrowing, zero-extend on
// widening from an unsigned source. Returns 0 on success, or a distinct code per failure.
function main(): i32 {
  // float -> int truncates toward zero
  let a: f64 = 3.9;
  if ((a as i32) != 3) { return 1; }

  // negative float -> int truncates toward zero: -3.9 -> -3 (so (-3) + 3 == 0)
  let n: f64 = 0.0 - 3.9;
  if (((n as i32) + 3) != 0) { return 2; }

  // int narrowing wraps: 300 mod 256 = 44
  let b: i32 = 300;
  if (((b as u8) as i32) != 44) { return 3; }

  // 257 mod 256 = 1
  let c: i32 = 257;
  if (((c as u8) as i32) != 1) { return 4; }

  // widening from an unsigned source zero-extends: u8 200 -> i32 200 (not -56)
  let d: u8 = 200 as u8;
  if ((d as i32) != 200) { return 5; }

  // same-width reinterpret is a no-op on the bits
  let e: i32 = 42;
  if (((e as u32) as i32) != 42) { return 6; }

  return 0;
}
