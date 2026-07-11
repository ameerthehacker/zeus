// A three-level chain where each level's constructor chains to its base via super(...).
class A {
  public a: i32;
  constructor(a: i32) { this.a = a; }
}
class B extends A {
  public b: i32;
  constructor(a: i32, b: i32) {
    super(a);
    this.b = b;
  }
}
class C extends B {
  public c: i32;
  constructor(a: i32, b: i32, c: i32) {
    super(a, b);
    this.c = c;
  }
}
function main(): i32 {
  let c: C = new C(20, 15, 7);
  return c.a + c.b + c.c; // 42
}
