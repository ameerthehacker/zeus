// BUG (wiki/compiler-bugs.md "Argument-count errors point at the callee's definition"):
// the "expected 2 arguments" error anchors to add's declaration (line ~9) instead of the
// bad call below.
function add(a: i32, b: i32): i32 {
  return a + b;
}
function main(): i32 {
  return add(1, 2, 3);
}
