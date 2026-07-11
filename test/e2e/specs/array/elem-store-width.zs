// Regression: a primitive array element write (arr[i] = v) must store at the ELEMENT
// width, not the width of the value expression. A narrow literal (e.g. 7, typed i8)
// written into an i32[] slot must emit a full 32-bit store — a 1-byte store would
// corrupt the upper 3 bytes (only observable when those bytes are non-zero, e.g. after
// fill(-1)). Also covers auto-extend on out-of-range writes and multiple writes per block.
function main(): i32 {
  // i32[]: narrow-literal stores after fill(-1); two writes in one block.
  let d: i32[] = new i32[4];
  d.fill(-1);
  d[0] = 7;
  d[1] = 8;
  if (d[0] != 7) { return 1; }
  if (d[1] != 8) { return 2; }
  if (d[2] != -1) { return 3; }

  // Out-of-range write must auto-extend the array (not throw, not corrupt).
  let e: i32[] = new i32[2];
  e[0] = 100;
  e[5] = 500;
  if (e.length < 6) { return 4; }
  if (e[5] != 500) { return 5; }
  if (e[0] != 100) { return 6; }

  // u8[]: byte-width stores stay correct.
  let b: u8[] = new u8[3];
  b.fill(9);
  b[1] = 200;
  if (b[1] != 200) { return 7; }
  if (b[0] != 9) { return 8; }

  return 0;
}
