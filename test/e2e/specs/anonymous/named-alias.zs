function main(): i32 {
  const Vec = class Vector {
    public x: i32;
    public y: i32;
    constructor(x: i32, y: i32) { this.x = x; this.y = y; }
    sum(): i32 { return this.x + this.y; }
  }
  let a = new Vec(3, 4);
  let b = new Vector(1, 2);
  return a.sum() + b.sum();
}
