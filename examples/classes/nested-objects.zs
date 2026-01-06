// Nested Objects Example
// Demonstrates: objects containing other objects, deep property access

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
  public start: Point;
  public end: Point;

  constructor(id: i32, p1: Point, p2: Point) {
    this.id = id;
    this.start = p1;
    this.end = p2;
  }

  public length(): i32 {
    // Simplified: Manhattan distance
    let dx: i32 = this.end.x - this.start.x;
    let dy: i32 = this.end.y - this.start.y;
    
    if (dx < 0) { dx = dx * -1; }
    if (dy < 0) { dy = dy * -1; }
    
    return dx + dy;
  }

  public midpointX(): i32 {
    return (this.start.x + this.end.x) / 2;
  }

  public midpointY(): i32 {
    return (this.start.y + this.end.y) / 2;
  }
}

class Triangle {
  public a: Point;
  public b: Point;
  public c: Point;

  constructor(p1: Point, p2: Point, p3: Point) {
    this.a = p1;
    this.b = p2;
    this.c = p3;
  }

  // Sum of all coordinates
  public coordinateSum(): i32 {
    return this.a.x + this.a.y + 
           this.b.x + this.b.y + 
           this.c.x + this.c.y;
  }
}

function main(): i32 {
  // Create points
  let p1: Point = new Point(0, 0);
  let p2: Point = new Point(10, 0);
  let p3: Point = new Point(5, 10);
  
  // Create a line
  let line: Line = new Line(1, p1, p2);
  let lineLength: i32 = line.length();  // 10
  
  // Create a triangle
  let triangle: Triangle = new Triangle(p1, p2, p3);
  let coordSum: i32 = triangle.coordinateSum();  // 0+0+10+0+5+10 = 25
  
  return lineLength + coordSum;  // 10 + 25 = 35
}

