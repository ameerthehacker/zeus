// One interface + one dispatch site, two conformers with DIFFERENT property backings: a field
// (Square.area) and an accessor (Circle.area). The tagged property itable routes each to its own
// backing at runtime — a direct offset load for the field, a getter call for the accessor.
interface HasArea {
  readonly area: i32;
}

class Square {
  public area: i32;                                  // field-backed
  constructor(a: i32) { this.area = a; }
}

class Circle {
  private r: i32;
  constructor(r: i32) { this.r = r; }
  get area(): i32 { return this.r * this.r; }        // accessor-backed
}

function areaOf(h: HasArea): i32 {
  return h.area;   // same dispatch site for both backings
}

function main(): i32 {
  let s: HasArea = new Square(5);   // field → 5
  let c: HasArea = new Circle(4);   // getter → 16
  return areaOf(s) + areaOf(c);     // 21
}
