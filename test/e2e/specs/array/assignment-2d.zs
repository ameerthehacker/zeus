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

  // Test 2D array assignment using bracket notation
  _2dArray[0][0] = new Point(1, 1);
  _2dArray[1][0] = new Point(2, 2);
  _2dArray[1][1] = new Point(3, 3);

  // Verify values were set correctly: _2dArray[0][0].sum() + _2dArray[1][0].sum() + _2dArray[1][1].sum()
  return _2dArray[0][0].sum() + _2dArray[1][0].sum() + _2dArray[1][1].sum();
}

