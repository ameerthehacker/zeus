// The class (and its variadic method) is declared AFTER it is used, so `s` resolves
// through the DeclCheckPass stub — the stub must still see the method as variadic.
function main(): i32 {
  let s: Summer = new Summer(10);
  if (s.sum(1, 2, 3) != 16) {
    return 1;
  }
  if (s.sum() != 10) {
    return 2;
  }
  return 0;
}

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
