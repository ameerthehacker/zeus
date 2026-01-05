function sum(nums: u8[]): u32 {
  let sum: u32 = 0;
  let i: u8 = 0;

  while (i < nums.length) {
    sum = sum + nums[i];
    i = i + 1;
  }
  
  return sum;
}

function main(): u32 {
  let nums: u8[] = new u8[];
  nums[0] = 1;
  nums[1] = 4;
  nums[2] = 6;

  return sum(nums);
}
