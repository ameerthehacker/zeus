function main(): u32 {
  let nums: u8[] = new u8[];
  let sum: u32 = 0;

  nums[0] = 1;
  nums[1] = 4;
  nums[2] = 6;

  let i: u8 = 0;

  while (i < nums.length) {
    sum = sum + nums[i];
    i = i + 1;
  }

  return sum;
}
