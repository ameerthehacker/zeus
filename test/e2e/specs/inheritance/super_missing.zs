// A derived constructor must call super(...) when the base has a constructor.
class Base {
  public a: i32;
  constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
  public b: i32;
  constructor(b: i32) {
    let x: i32 = b + 1; // no super(...) -> compile error
  }
}
function main(): i32 {
  let d: Derived = new Derived(1);
  return d.b;
}
