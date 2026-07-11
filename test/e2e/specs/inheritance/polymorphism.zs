// A derived instance is passed where a base type is expected (upcast), and the virtual
// method still dispatches to the derived override.
class Shape {
  public area(): i32 { return 0; }
}
class Square extends Shape {
  public s: i32;
  constructor(s: i32) { this.s = s; }
  public area(): i32 { return this.s * this.s; }
}
function describe(sh: Shape): i32 { return sh.area(); }
function main(): i32 {
  let sq: Square = new Square(6);
  return describe(sq) + 6; // 36 + 6 = 42
}
