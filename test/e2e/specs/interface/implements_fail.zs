interface Shape {
  area(): i32;
}

// Square declares it implements Shape but lacks area() — a compile error.
class Square implements Shape {
  public side: i32;
  constructor(s: i32) { this.side = s; }
}

function main(): i32 {
  return 0;
}
