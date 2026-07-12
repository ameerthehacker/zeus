interface HasArea {
  area: i32;
}

// Circle exposes `area` only via an accessor (get), not a stored field or method.
// Interface dispatch resolves properties by field offset, which an accessor doesn't
// provide, so Circle must NOT be accepted as a HasArea (a clean error, not a crash).
class Circle {
  public r: i32;
  constructor(r: i32) { this.r = r; }
  get area(): i32 { return this.r * this.r; }
}

function use(h: HasArea): i32 {
  return 0;
}

function main(): i32 {
  let c: Circle = new Circle(5);
  return use(c);
}
