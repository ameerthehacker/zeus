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

class X {
  x: i32;

  constructor(x: i32) {
    this.x = x;
  }

  public getX(): i32 {
    return this.x;
  }
}

function main(): i32 {
  let array1: Point[] = new Point[];
  const array2: X[] = new X[];

  array1.push(new Point(1, 2));
  array1.push(new Point(3, 3));
  array2.push(new X(2));

  return array1.get(1).sum() + array1.get(0).sum() + array2.get(0).getX();
}
