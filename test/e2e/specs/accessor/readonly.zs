class Id {
  private n: i32;
  constructor(n: i32) { this.n = n; }
  get id(): i32 { return this.n; }   // getter only
}
function main(): i32 {
  let x: Id = new Id(1);
  x.id = 5;   // no setter -> compile error
  return 0;
}
