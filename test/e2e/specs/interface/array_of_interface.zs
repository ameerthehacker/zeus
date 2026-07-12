// An array whose element type is an interface: interface values are object pointers, so an
// interface array shares the object-array layout. Elements dispatch dynamically to their
// concrete class, so this exercises get()/push() plus itable dispatch on array elements.
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

function main(): i32 {
  let shapes: Shape[] = new Shape[];
  shapes.push(new Circle(3));   // area 9
  shapes.push(new Square(5));   // area 20
  shapes.push(new Circle(2));   // area 4

  let total: i32 = 0;
  let i: i32 = 0;
  while (i < 3) {
    total = total + shapes.get(i).area();
    i = i + 1;
  }
  return total;   // 9 + 20 + 4 = 33
}
