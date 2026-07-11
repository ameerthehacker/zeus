// Getter/setter accessors are inherited and dispatched through the derived object.
class Base {
  public _v: i32;
  constructor(v: i32) { this._v = v; }
  get value(): i32 { return this._v; }
  set value(x: i32) { this._v = x; }
}
class Derived extends Base {
  constructor(v: i32) { super(v); }
}
function main(): i32 {
  let d: Derived = new Derived(40);
  d.value = d.value + 2; // inherited setter + getter
  return d.value;        // 42
}
