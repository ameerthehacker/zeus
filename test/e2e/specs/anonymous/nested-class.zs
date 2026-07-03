function makePoint(x: i32, y: i32): i32 {
  const Point = class {
    public x: i32;
    public y: i32;
    constructor(x: i32, y: i32) { this.x = x; this.y = y; }
    sum(): i32 { return this.x + this.y; }
  }
  let p = new Point(x, y);
  return p.sum();
}

function main(): i32 {
  return makePoint(3, 7);
}
