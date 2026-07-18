// BUG (wiki "Accessor access on an array-subscript receiver fails"): arr[i].getter reports
// "property not found"; via a local it works. When fixed, this returns 0.
class C {
  private _v: i32;
  constructor(v: i32) { this._v = v; }
  get v(): i32 { return this._v; }
}
function main(): i32 {
  let a: C[] = new C[];
  a.push(new C(42));
  return a[0].v - 42;
}
