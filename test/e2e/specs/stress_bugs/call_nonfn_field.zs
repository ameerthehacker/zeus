// BUG (wiki/compiler-bugs.md "Calling a non-function field reports property not found"):
// invoking a data field says "property x not found" although x exists (it is just an i32,
// not callable).
class A {
  public x: i32;
  constructor() { this.x = 1; }
}
function main(): i32 {
  let a: A = new A();
  return a.x();
}
