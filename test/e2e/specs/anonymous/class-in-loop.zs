function main(): i32 {
  let total: i32 = 0;
  let i: i32 = 0;
  while (i < 3) {
    const Counter = class {
      public v: i32;
      constructor(v: i32) { this.v = v; }
      get(): i32 { return this.v; }
    }
    total = total + new Counter(i).get();
    i = i + 1;
  }
  return total;
}
