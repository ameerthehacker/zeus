interface Calc {
  base(): i32;
  scale(f: i32): i32;
}

class Doubler {
  public n: i32;
  constructor(n: i32) { this.n = n; }
  public base(): i32 { return this.n; }
  public scale(f: i32): i32 { return this.n * f; }
}

// Exercises multiple itable slots (base = slot 0, scale = slot 1) and passing an
// argument through a dynamically dispatched interface method.
function compute(c: Calc): i32 {
  return c.base() + c.scale(2);
}

function main(): i32 {
  let d: Doubler = new Doubler(14);
  return compute(d);   // base=14 + scale(2)=28 = 42
}
