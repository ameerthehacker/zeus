// REGRESSION (wiki/compiler-bugs.md, FIXED 2026-07-15 PR #28): two same-named locals `n`
// in sibling if-branches used to collide on their IR name and crash/miscompile once an
// array-index bounds-check split a branch across basic blocks. Now compiles and returns 0.
class N { public v: i32; constructor() { this.v = 0; } }
function pick(a: i32[], k: i32): N {
  if (k == 1) { let n: N = new N(); n.v = a[0]; return n; }
  if (k == 2) { let n: N = new N(); n.v = a[1]; return n; }
  return new N();
}
function main(): i32 {
  let a: i32[] = [10, 20];
  return pick(a, 1).v - 10;
}
