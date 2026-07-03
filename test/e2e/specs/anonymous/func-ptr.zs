function double(x: i32): i32 {
  return x * 2;
}

function main(): i32 {
  let f = double;
  return f(5);
}
