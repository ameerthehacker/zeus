interface Shape {
  area(): i32;
}

// Square has no area() method, so it does NOT structurally conform to Shape.
class Square {
  public side: i32;

  constructor(s: i32) {
    this.side = s;
  }
}

function useShape(s: Shape): i32 {
  return 1;
}

function main(): i32 {
  let sq: Square = new Square(5);

  return useShape(sq);
}
