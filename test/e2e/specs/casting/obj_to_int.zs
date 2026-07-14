// An object cannot be cast to a number — rejected at compile time.
class Foo {
  public x: i32;
  constructor() { this.x = 1; }
}

function main(): i32 {
  let f: Foo = new Foo();
  return f as i32;
}
