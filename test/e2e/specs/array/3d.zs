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
  let _3dArray: Point[][][] = new Point[][][];
  
  // Create first 2D array
  _3dArray.push(new Point[][]);
  _3dArray.get(0).push(new Point[10]);
  _3dArray.get(0).push(new Point[10]);
  _3dArray.get(0).get(0).push(new Point(1, 1));
  _3dArray.get(0).get(1).push(new Point(2, 2));
  
  // Create second 2D array
  _3dArray.push(new Point[][]);
  _3dArray.get(1).push(new Point[10]);
  _3dArray.get(1).get(0).push(new Point(3, 3));
  _3dArray.get(1).get(0).push(new Point(4, 4));
  
  // Access and sum: [0][0][0] + [0][1][0] + [1][0][0] + [1][0][1]
  return _3dArray.get(0).get(0).get(0).sum() + 
         _3dArray.get(0).get(1).get(0).sum() + 
         _3dArray.get(1).get(0).get(0).sum() + 
         _3dArray.get(1).get(0).get(1).sum();
}