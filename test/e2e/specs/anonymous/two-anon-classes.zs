function makeAdder(): i32 {
  const Adder = class {
    public a: i32;
    public b: i32;
    constructor(a: i32, b: i32) { this.a = a; this.b = b; }
    compute(): i32 { return this.a + this.b; }
  }
  return new Adder(3, 4).compute();
}

function makeMultiplier(): i32 {
  const Multiplier = class {
    public a: i32;
    public b: i32;
    constructor(a: i32, b: i32) { this.a = a; this.b = b; }
    compute(): i32 { return this.a * this.b; }
  }
  return new Multiplier(1, 3).compute();
}

function main(): i32 {
  return makeAdder() + makeMultiplier();
}
