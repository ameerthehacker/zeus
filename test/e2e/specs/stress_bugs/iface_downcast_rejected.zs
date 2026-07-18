// REGRESSION (fixed): an interface value can now be downcast to a concrete implementer via `as`,
// verified at runtime (ClassCastException on mismatch) like a class->class downcast.
interface Shape { area(): i32; }
class Circle {
  public r: i32;
  constructor(r: i32) { this.r = r; }
  public area(): i32 { return this.r * this.r; }
  public radius(): i32 { return this.r; }
}
function main(): i32 {
  let s: Shape = new Circle(5);
  let c: Circle = s as Circle;
  return c.area() + c.radius() - 30;
}
