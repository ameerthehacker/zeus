// Regression: two sibling `if` branches each declared a local of the SAME name
// (`let n`) and returned an object, with the branch bodies split across multiple
// basic blocks by array-index bounds checks. The two locals used to receive the
// same IR name, so codegen's name-keyed value map collided — a later branch's load
// resolved to the wrong (uninitialized) alloca. That crashed LLVM object emission
// (register coalescer) in debug builds and miscompiled to a null return in release.
// See internal/ir/builder.go generateUniqueVarIRName.

class N {
  public v: i32;
  constructor() {
    this.v = 0;
  }
}

function pick(a: i32[], k: i32): N {
  if (k == 1) { let n: N = new N(); n.v = a[0]; return n; }
  if (k == 2) { let n: N = new N(); n.v = a[1]; return n; }
  return new N();
}

function main(): i32 {
  let a: i32[] = [10, 20];
  // pick(a, 1).v == 10, property access directly on the call result.
  return pick(a, 1).v - 10;
}
