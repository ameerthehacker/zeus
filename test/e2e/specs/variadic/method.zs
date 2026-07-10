class Summer {
  public base: i32;

  constructor(base: i32) {
    this.base = base;
  }

  public sum(...nums: i32[]): i32 {
    let total: i32 = this.base;
    for (let i: i32 = 0; i < nums.length; i++) {
      total += nums[i];
    }
    return total;
  }
}

function main(): i32 {
  let s = new Summer(10);
  if (s.sum(1, 2, 3, 4) != 20) {
    return 1;
  }
  // Empty variadic method call.
  if (s.sum() != 10) {
    return 2;
  }
  return 0;
}
