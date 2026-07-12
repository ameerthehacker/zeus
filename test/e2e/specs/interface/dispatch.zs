interface Shape {
  area(): i32;
}

class Circle {
  public r: i32;
  constructor(r: i32) { this.r = r; }
  public area(): i32 { return this.r * this.r; }
}

class Square {
  public side: i32;
  constructor(s: i32) { this.side = s; }
  public area(): i32 { return this.side * 4; }
}

// Polymorphic: areaOf dispatches s.area() to whatever concrete class is passed in.
function areaOf(s: Shape): i32 {
  return s.area();
}

function main(): i32 {
  let c: Circle = new Circle(5);   // area = 25
  let sq: Square = new Square(4);  // area = 16

  return areaOf(c) + areaOf(sq);   // 25 + 16 = 41
}
