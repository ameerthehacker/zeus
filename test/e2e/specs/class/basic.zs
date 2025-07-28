class Example {
  public x: i32;
}

class Point {
  public x: i32;
  public example: Example;

  constructor(x: i32) {
    this.x = x;
    this.example = new Example();
  }
}

function main(): i32 {
 let p: Point = new Point(10);

 return p.example.x + 10;
}
