interface Animal {
  legs(): i32;
}

interface Pet extends Animal {
  noise(): i32;
}

class Dog {
  public l: i32;
  constructor() { this.l = 4; }
  public legs(): i32 { return this.l; }
  public noise(): i32 { return 38; }
}

// Dispatches both an own method (noise) and an inherited one (legs, from Animal via
// `extends`) through the Pet interface — the itable must use the flattened method order.
function describe(p: Pet): i32 {
  return p.legs() + p.noise();
}

function main(): i32 {
  let d: Dog = new Dog();
  return describe(d);   // 4 + 38 = 42
}
