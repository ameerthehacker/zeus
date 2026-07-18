// BUG (wiki/compiler-bugs.md "Docs function-type syntax is wrong"): types.mdx shows
// `let op: function(i32, i32): i32 = add;` but the real syntax is `(i32, i32) => i32`.
function add(a: i32, b: i32): i32 { return a + b; }
function main(): i32 {
  let operation: function(i32, i32): i32 = add;
  return operation(2, 3) - 5;
}
