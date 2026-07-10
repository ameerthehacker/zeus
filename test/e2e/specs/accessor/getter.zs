class Circle {
  private r: i32;
  constructor(r: i32) { this.r = r; }
  // Computed getter: not backed by a field of the same name.
  get area(): i32 { return this.r * this.r; }
}
function main(): i32 {
  let c: Circle = new Circle(5);
  return c.area;   // 25
}
