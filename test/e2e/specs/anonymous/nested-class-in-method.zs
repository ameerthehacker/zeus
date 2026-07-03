function makeBox(): i32 {
  const Outer = class {
    public val: i32;
    constructor(v: i32) { this.val = v; }
    makeInner(): i32 {
      const Inner = class {
        public n: i32;
        constructor(n: i32) { this.n = n; }
        get(): i32 { return this.n; }
      }
      return new Inner(this.val + 1).get();
    }
  }
  return new Outer(9).makeInner();
}

function main(): i32 {
  return makeBox();
}
