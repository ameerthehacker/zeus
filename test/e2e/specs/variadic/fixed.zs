function sumFrom(base: i32, ...nums: i32[]): i32 {
  let total: i32 = base;
  for (let i: i32 = 0; i < nums.length; i++) {
    total += nums[i];
  }
  return total;
}

function main(): i32 {
  if (sumFrom(100, 1, 2, 3) != 106) {
    return 1;
  }
  if (sumFrom(50) != 50) {
    return 2;
  }
  return 0;
}
