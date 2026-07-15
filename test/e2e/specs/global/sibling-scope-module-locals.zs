// Regression: two module-scope locals with the SAME source name in sibling scopes
// (`let n` in two module-level `if` branches). These go through BuildGlobalVarDecl,
// which used to de-dupe the IR name only against in-scope symbols, so both got the
// name "n" and collided in codegen's name-keyed value map (globals share it). The
// inner `while` defers each branch's `n.v` read past a merge block, so the load
// resolved to the sibling branch's global (still null) -> NullReferenceException.
// See internal/ir/builder.go BuildGlobalVarDecl / usedGlobalNames.

class N {
  public v: i32;
  constructor() {
    this.v = 0;
  }
}

let a: i32[] = [10, 20];
let o1: i32 = 0;
let o2: i32 = 0;

if (a[0] == 10) {
  let n: N = new N();
  n.v = a[0];
  let j: i32 = 0;
  while (j < 1) { j = j + 1; }
  o1 = n.v;
}

if (a[1] == 20) {
  let n: N = new N();
  n.v = a[1];
  let j: i32 = 0;
  while (j < 1) { j = j + 1; }
  o2 = n.v;
}

function main(): i32 {
  return o1 + o2 - 30;
}
