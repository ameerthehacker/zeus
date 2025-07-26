class Point {
  public x: i32;
  public y: i32;

  constructor(x: i32, y: i32) {
    this.x = x;
    this.y = y;
  }
}

class Line {
  public id: i32;
  public p1: Point;
  public p2: Point;

  constructor(id: i32, p1: Point, p2: Point) {
    this.id = id;
    this.p1 = p1;
    this.p2 = p2;
  }
}

function dummy(): void {}

function main(): i32 {
  let p1: Point = new Point(10, 20);
  let p2: Point = new Point(30, 20);
  let l: Line = new Line(1, p1, p2);
  new Line(2, new Point(10, 20), new Point(30, 20));

  // force a gc cycle
  dummy();
  return l.p1.x + l.p2.y;
}
