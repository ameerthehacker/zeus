function makeDouble(): (x: i32) => i32 {
  return (x: i32): i32 => { return x * 2; };
}

function apply(f: (x: i32) => i32, v: i32): i32 {
  return f(v);
}

function main(): i32 {
  return apply(makeDouble(), 5);
}
