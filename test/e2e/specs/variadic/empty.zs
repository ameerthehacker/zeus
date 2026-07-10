function tally(base: i32, ...nums: i32[]): i32 {
  return base + nums.length;
}

function allVariadic(...nums: i32[]): i32 {
  return nums.length;
}

function main(): i32 {
  // No trailing args -> empty rest array (length 0).
  if (tally(5) != 5) {
    return 1;
  }
  // Zero fixed params and zero trailing args -> empty rest array.
  if (allVariadic() != 0) {
    return 2;
  }
  if (allVariadic(7, 8, 9) != 3) {
    return 3;
  }
  return 0;
}
