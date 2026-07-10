// Regression: a top-level function assigned to a function-typed property (or returned as
// a function type) must be wrapped in a functor object, not stored as a raw function.
function add(a: i32, b: i32): i32 {
  return a + b;
}

class Holder {
  public op: (a: i32, b: i32) => i32;

  constructor() {
    this.op = add;
  }
}

function makeAdd(): (a: i32, b: i32) => i32 {
  return add;
}

function main(): i32 {
  let h = new Holder();
  if (h.op(3, 4) != 7) {
    return 1;
  }
  // Reassign the property outside the constructor.
  h.op = makeAdd();
  if (h.op(10, 20) != 30) {
    return 2;
  }
  return 0;
}
