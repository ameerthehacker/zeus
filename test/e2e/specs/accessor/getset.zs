class Temp {
  private c: i32;
  constructor(c: i32) { this.c = c; }
  get celsius(): i32 { return this.c; }
  set celsius(v: i32) { this.c = v; }
}
function main(): i32 {
  let t: Temp = new Temp(20);
  let a: i32 = t.celsius;   // getter -> 20
  t.celsius = 22;           // setter -> 22
  return a + t.celsius;     // 42
}
