// super.method() resolves up the chain: C's super skips B (which doesn't define v) and
// reaches A's implementation.
class A {
  public v(): i32 { return 40; }
}
class B extends A { }
class C extends B {
  public v(): i32 { return super.v() + 2; }
}
function main(): i32 {
  let c: C = new C();
  return c.v(); // super.v() finds A.v via B -> 40 + 2 = 42
}
