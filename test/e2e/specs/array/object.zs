class Point {
  x: i32;
  y: i32;

  constructor(x: i32, y: i32) {
    this.x = x;
    this.y = y;
  }

  public sum(): i32 {
    return this.x + this.y;
  }
}

function main(): i32 {
  let array: Point[] = new Point[10];

  array.push(new Point(1, 2));
  array.push(new Point(3, 3));

  return array.get(1).sum() + array.get(0).sum();
}
