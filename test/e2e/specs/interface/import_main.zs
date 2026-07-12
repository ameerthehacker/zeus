import { Shape } from "./shapes";

class Circle {
  public radius: i32;

  constructor(r: i32) {
    this.radius = r;
  }

  public area(): i32 {
    return this.radius;
  }
}

function useShape(s: Shape): i32 {
  return 0;
}

function main(): i32 {
  let c: Circle = new Circle(42);

  return useShape(c) + c.radius;
}
