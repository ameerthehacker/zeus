interface Shape {
  radius: i32;
  area(): i32;
}

class Circle {
  public radius: i32;

  constructor(r: i32) {
    this.radius = r;
  }

  public area(): i32 {
    return this.radius;
  }
}

// Circle structurally conforms to Shape (radius: i32 + area(): i32), so it is
// accepted where a Shape is expected. Member access through the interface is not
// yet supported, so the concrete value is read directly.
function describe(s: Shape): i32 {
  return 7;
}

function main(): i32 {
  let c: Circle = new Circle(35);

  return describe(c) + c.radius;
}
