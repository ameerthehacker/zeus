interface HasArea {
  area: i32;
}

// `area: i32` is a WRITABLE interface property, so a conformer must be able to write it — a
// real field, or an accessor with BOTH a getter and a setter. Circle has a getter-only accessor,
// so it does NOT conform (a clean error, not a crash). (A `readonly area` would accept it.)
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
