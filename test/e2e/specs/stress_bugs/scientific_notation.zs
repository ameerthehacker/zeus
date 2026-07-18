// BUG (wiki/compiler-bugs.md "Scientific-notation float literals do not parse"): types.mdx
// lists 6.022e23 as a supported literal format, but the lexer does not accept it.
function main(): i32 {
  let x: f64 = 6.022e23;
  if (x > 0.0) { return 0; }
  return 1;
}
