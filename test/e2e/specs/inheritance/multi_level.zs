// Multi-level chain: C -> B -> A. B overrides who(); C inherits it. A's gift() is
// inherited all the way down to C and dispatched through an A-typed reference.
class A {
  public who(): i32 { return 1; }
  public gift(): i32 { return 40; }
}
class B extends A {
  public who(): i32 { return 2; }
}
class C extends B { }
function main(): i32 {
  let c: A = new C();
  return c.who() + c.gift(); // 2 (B's override) + 40 (A's inherited) = 42
}
