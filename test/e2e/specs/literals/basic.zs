// Numeric-literal typing: integer literals are signed (i32-floored) and adopt a narrower/target
// type when the value fits; float literals stay f64 and adopt f32. Returns 0 on success.
function main(): i32 {
  // Negative arithmetic no longer wraps (previously 0 - 4 computed in u8 -> 252).
  let a: i32 = 0 - 4;
  if (a + 4 != 0) { return 1; }

  // The exact original repro: 5 > (0 - 4) must be true, not false.
  if (!(5 > (0 - 4))) { return 2; }

  // A literal adopts a narrower target type when it fits.
  let b: u8 = 200;
  if ((b as i32) != 200) { return 3; }

  // Context-less literal defaults to i32 (no i8 overflow at 500).
  let x = 5;
  if (x * 100 / 50 != 10) { return 4; }

  // A literal combined with a typed var keeps that var's type (b + 1 stays u8).
  let c: u8 = b + 1;
  if ((c as i32) != 201) { return 5; }

  // Float literal adopts the f32 target.
  let f: f32 = 2.0;
  if ((f as f64) != 2.0) { return 6; }

  // Bitwise still works on integer literals.
  if ((5 & 3) != 1) { return 7; }
  if ((0x0F | 0x30) != 0x3F) { return 8; }

  return 0;
}
