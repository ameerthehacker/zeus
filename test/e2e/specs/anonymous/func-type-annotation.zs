function add(a: i32, b: i32): i32 {
  return a + b;
}

function apply(f: (a: i32, b: i32) => i32, x: i32, y: i32): i32 {
  return f(x, y);
}

function identity(g: (x: i32) => i32): (x: i32) => i32 {
  return g;
}

function main(): i32 {
  let result: i32 = apply(add, 3, 7);
  let double = function(x: i32): i32 { return x * 2; };
  let op: (x: i32) => i32 = double;
  let op2: (x: i32) => i32 = identity(op);
  return result - op2(5);
}
