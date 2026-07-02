class Calc {
  public add(a: i32, b: i32): i32 { return a + b; }
}

function apply(f: (a: i32, b: i32) => i32, x: i32, y: i32): i32 {
  return f(x, y);
}

function main(): i32 {
  let c = new Calc();
  return apply(c.add, 3, 4);
}
