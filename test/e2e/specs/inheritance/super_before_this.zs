// `this` may not be used before super(...) in a derived constructor (JS semantics).
class Base {
  public a: i32;
  constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
  public b: i32;
  constructor(a: i32, b: i32) {
    this.b = b;  // uses `this` before super(...) -> compile error
    super(a);
  }
}
function main(): i32 {
  let d: Derived = new Derived(1, 2);
  return d.a;
}
