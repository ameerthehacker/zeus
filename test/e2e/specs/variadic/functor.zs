// A functor (class with __call__) taking variadic arguments, invoked as `s(...)`.
class Sum {
  public __call__(...nums: i32[]): i32 {
    let total: i32 = 0;
    for (let i: i32 = 0; i < nums.length; i++) {
      total += nums[i];
    }
    return total;
  }
}

function main(): i32 {
  const s = new Sum();
  if (s(1, 2, 3) != 6) {
    return 1;
  }
  // Empty variadic functor call.
  if (s() != 0) {
    return 2;
  }
  if (s(10) != 10) {
    return 3;
  }
  return 0;
}
