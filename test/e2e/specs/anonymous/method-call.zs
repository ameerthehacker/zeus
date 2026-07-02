class Calc {
  public add(a: i32, b: i32): i32 { return a + b; }
  public mul(a: i32, b: i32): i32 { return a * b; }
}

function main(): i32 {
  let c = new Calc();
  return c.add(c.mul(2, 3), 1);
}
