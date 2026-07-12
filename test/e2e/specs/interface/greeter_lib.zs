export interface Greeter {
  greet(): i32;
}

export class English {
  public base: i32;
  constructor(b: i32) { this.base = b; }
  public greet(): i32 { return this.base; }
}
