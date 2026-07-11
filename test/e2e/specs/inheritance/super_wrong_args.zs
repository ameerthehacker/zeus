// super(...) arguments must match the base constructor's parameters.
class Base {
  public a: i32;
  constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
  public b: i32;
  constructor(b: i32) {
    super();     // base constructor expects 1 argument -> compile error
    this.b = b;
  }
}
function main(): i32 {
  let d: Derived = new Derived(2);
  return d.b;
}
