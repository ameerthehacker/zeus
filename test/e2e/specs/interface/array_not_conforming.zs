// Object arrays (Point[], Shape[], ...) share ONE physical Object[] layout — struct, vtable,
// and typeInfo — so at runtime they all report the same class id. The per-class interface
// itable is keyed by class id, so an object array can't be dispatched through it. To keep this
// sound, arrays are excluded from interface conformance: even though Point[] structurally has
// `push`/`get`, it is rejected at compile time (a clean type error), never a wrong dispatch.
class Point {
  public x: i32;
  constructor(x: i32) { this.x = x; }
}

interface Container {
  push(value: Point): void;
  get(index: i32): Point;
}

function firstX(c: Container): Point {
  return c.get(0);
}

function main(): i32 {
  let a: Point[] = new Point[];
  a.push(new Point(7));
  let c: Container = a;   // error: 'Point[]' is not assignable to type 'Container'
  return firstX(c).x;
}
