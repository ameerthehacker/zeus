// float <-> float widening and narrowing `as` casts.
function main(): i32 {
  // f64 -> f32 narrowing, then back to f64 (3.0 is exact in both)
  let a: f32 = 3.0 as f32;
  if ((a as f64) != 3.0) { return 1; }

  // f32 -> f64 widening preserves an exact value
  let b: f32 = 2.5 as f32;
  if ((b as f64) != 2.5) { return 2; }

  // chained casts: f64 -> f32 -> i32
  let c: f64 = 7.8;
  if (((c as f32) as i32) != 7) { return 3; }

  return 0;
}
