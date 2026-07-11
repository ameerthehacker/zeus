// A derived class can read and write both inherited and own fields.
class Base {
  public a: i32;
  constructor(a: i32) { this.a = a; }
}
class Derived extends Base {
  public b: i32;
  constructor(a: i32, b: i32) {
    this.a = a; // inherited field
    this.b = b; // own field
  }
}
function main(): i32 {
  let d: Derived = new Derived(40, 2);
  return d.a + d.b; // 42
}
