// Casting between classes in unrelated hierarchies is rejected at compile time.
class Foo {
  public x: i32;
  constructor() { this.x = 1; }
}

class Bar {
  public y: i32;
  constructor() { this.y = 2; }
}

function main(): i32 {
  let f: Foo = new Foo();
  let b: Bar = f as Bar;  // unrelated classes -> compile error
  return b.y;
}
