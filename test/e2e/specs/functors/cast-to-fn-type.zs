class Doubler {
  public __call__(x: i32): i32 {
    return x * 2;
  }
}

function apply(f: (x: i32) => i32, v: i32): i32 {
  return f(v);
}

function main(): i32 {
  const d = new Doubler();
  const op: (x: i32) => i32 = d;
  const result: i32 = apply(op, 5);
  if (result != 10) {
    return 1;
  }
  return 0;
}
