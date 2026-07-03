function main(): i32 {
  const Counter = class {
    public n: i32;
    constructor(start: i32) { this.n = start; }
    inc(): i32 { this.n = this.n + 1; return this.n; }
  }
  let c = new Counter(9);
  return c.inc();
}
