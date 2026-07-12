// An object array conforms to an interface it structurally matches. Each element type (Point[]) gets
// its own runtime type handle (a distinct class id) while sharing the Object[] code, so it can be
// keyed in the interface dispatch table. This exercises BOTH property dispatch (readonly length, a
// field) and method dispatch (get) through an interface value backed by an array.
class Point {
  public x: i32;
  constructor(x: i32) { this.x = x; }
}

interface Container {
  readonly length: i32;
  get(index: i32): Point;
}

function sumFirstTwoX(c: Container): i32 {
  return c.length * 100 + c.get(0).x + c.get(1).x;
}

function main(): i32 {
  let a: Point[] = new Point[];
  a.push(new Point(7));
  a.push(new Point(9));
  let c: Container = a;   // Point[] conforms to Container
  return sumFirstTwoX(c); // length 2 -> 2*100 + 7 + 9 = 216
}
