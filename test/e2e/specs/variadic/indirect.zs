function sum(...nums: i32[]): i32 {
  let total: i32 = 0;
  for (let i: i32 = 0; i < nums.length; i++) {
    total += nums[i];
  }
  return total;
}

function main(): i32 {
  // A variadic function used as a first-class value, called indirectly.
  let f: (...nums: i32[]) => i32 = sum;
  if (f(2, 4, 6) != 12) {
    return 1;
  }
  return 0;
}
