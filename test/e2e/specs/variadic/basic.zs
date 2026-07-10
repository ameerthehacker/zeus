function sum(...nums: i32[]): i32 {
  let total: i32 = 0;
  for (let i: i32 = 0; i < nums.length; i++) {
    total += nums[i];
  }
  return total;
}

function main(): i32 {
  if (sum(1, 2, 3, 4, 5) != 15) {
    return 1;
  }
  if (sum(10, 20) != 30) {
    return 2;
  }
  return 0;
}
