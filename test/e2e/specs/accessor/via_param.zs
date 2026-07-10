class Box {
  private v: i32;
  constructor(v: i32) { this.v = v; }
  get value(): i32 { return this.v; }
}
// Accessor dispatched on a receiver whose concrete type is only known via the parameter type.
function readIt(b: Box): i32 { return b.value; }
function main(): i32 {
  let b: Box = new Box(42);
  return readIt(b);
}
