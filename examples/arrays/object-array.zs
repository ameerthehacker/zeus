// Object Arrays
// Demonstrates: arrays containing class instances

class Point {
  public x: i32;
  public y: i32;

  constructor(x: i32, y: i32) {
    this.x = x;
    this.y = y;
  }

  public sum(): i32 {
    return this.x + this.y;
  }

  public distanceFromOrigin(): i32 {
    // Simplified: Manhattan distance
    let dx: i32 = this.x;
    let dy: i32 = this.y;
    if (dx < 0) { dx = dx * -1; }
    if (dy < 0) { dy = dy * -1; }
    return dx + dy;
  }
}

function sumAllPoints(points: Point[], count: i32): i32 {
  let total: i32 = 0;
  let i: i32 = 0;
  
  while (i < count) {
    total = total + points.get(i).sum();
    i = i + 1;
  }
  
  return total;
}

function main(): i32 {
  // Create array of points
  let points: Point[] = new Point[];
  
  points.push(new Point(1, 2));   // sum = 3
  points.push(new Point(3, 4));   // sum = 7
  points.push(new Point(5, 6));   // sum = 11
  points.push(new Point(10, 10)); // sum = 20
  
  // Total sum should be 3 + 7 + 11 + 20 = 41
  return sumAllPoints(points, 4);
}

