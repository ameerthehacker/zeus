// REGRESSION (fixed): Array.slice now clamps start/end to [0, length] and never produces a
// negative length. slice(0,4) on a 3-element array clamps to length 3 (was: unclamped 4 with an
// out-of-bounds over-read); slice(4,1) is empty (was: SIGSEGV); slice(-5,2) clamps.
function main(): i32 {
  let a: i32[] = [1, 2, 3];
  let ok: i32 = 0;
  if ((a.slice(0, 4).length as i32) == 3) { ok = ok + 1; }
  if ((a.slice(4, 1).length as i32) == 0) { ok = ok + 2; }
  if ((a.slice(0 - 5, 2).length as i32) == 2) { ok = ok + 4; }
  if (ok == 7) { return 0; }
  return 1;
}
