function main(): i32 {
  class Point {
    public x: i32;
    public y: i32;

    constructor(x: i32, y: i32) {
      this.x = x;
      this.y = y;
    }

    sum(): i32 {
      return this.x + this.y;
    }
  }

  let p = new Point(3, 7);
  return p.sum();
}
