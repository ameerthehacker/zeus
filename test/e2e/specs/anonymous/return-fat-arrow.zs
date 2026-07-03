function makeDouble(): (x: i32) => i32 {
  return (x: i32): i32 => { return x * 2; }
}

function main(): i32 {
  let f = makeDouble();
  return f(5);
}
