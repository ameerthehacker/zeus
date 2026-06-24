function main(): i32 {
  let n: i32 = 5;
  let arr: i32[] = new i32[n];

  // Default reads must work immediately after construction (no .set() first)
  if (arr[0] != 0) { return 1; }
  if (arr[4] != 0) { return 2; }

  // arr.length must equal n
  if (arr.length != 5) { return 3; }

  // Writing via .set() and reading back must work
  arr.set(0, 10);
  arr.set(4, 20);
  if (arr[0] != 10) { return 4; }
  if (arr[4] != 20) { return 5; }

  // Parameterized size: function-local variable
  let m: i32 = 3;
  let b: i32[] = new i32[m];
  if (b.length != 3) { return 6; }
  if (b[2] != 0) { return 7; }

  return 0;
}
