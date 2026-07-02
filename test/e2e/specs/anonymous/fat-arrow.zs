function apply(f: (x: i32) => i32, x: i32): i32 {
  return f(x);
}

function main(): i32 {
  let add = (a: i32, b: i32): i32 => { return a + b; };
  let result: i32 = apply((x: i32): i32 => { return x + 1; }, 4);
  return add(result, 0) - 5;
}
