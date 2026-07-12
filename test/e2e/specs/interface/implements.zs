// The interface is declared AFTER the class; the conformance check is deferred until all
// declarations are resolved, so this still type-checks.
class Circle implements Shape {
  public radius: i32;
  constructor(r: i32) { this.radius = r; }
  public area(): i32 { return this.radius; }
}

interface Shape {
  radius: i32;
  area(): i32;
}

function main(): i32 {
  let c: Circle = new Circle(42);
  return c.radius;
}
