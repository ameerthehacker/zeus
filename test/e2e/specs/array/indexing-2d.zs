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
  let _2dArray: Point[][] = new Point[][];
  _2dArray.push(new Point[10]);
  _2dArray.push(new Point[10]);
  _2dArray.get(0).push(new Point(1, 1));
  _2dArray.get(1).push(new Point(2, 2));
  _2dArray.get(1).push(new Point(3, 3));

  // Test 2D array indexing: _2dArray[0][0].sum() + _2dArray[1][0].sum() + _2dArray[1][1].sum()
  return _2dArray[0][0].sum() + _2dArray[1][0].sum() + _2dArray[1][1].sum();
}

