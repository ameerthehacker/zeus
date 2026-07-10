class Inner {
  private v: i32;
  constructor(v: i32) { this.v = v; }
  get value(): i32 { return this.v; }
}
class Outer {
  public inner: Inner;
  constructor(i: Inner) { this.inner = i; }
}
function main(): i32 {
  let o: Outer = new Outer(new Inner(42));
  // Chained: field .inner (Inner) then accessor .value — the intermediate field's type
  // must be known at IR-gen for the accessor to resolve.
  return o.inner.value;
}
