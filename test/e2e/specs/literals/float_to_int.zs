// A float literal does not implicitly become an integer; use `2` or `2.0 as i32`.
function main(): i32 {
  let x: i32 = 2.0;
  return x;
}
