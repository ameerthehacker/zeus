// A derived class with no constructor of its own inherits the base constructor:
// `new Derived(args)` forwards to the base constructor.
class Base {
  public v: i32;
  constructor(v: i32) { this.v = v; }
}
class Derived extends Base { }
function main(): i32 {
  let d: Derived = new Derived(42);
  return d.v; // base constructor ran -> 42
}
