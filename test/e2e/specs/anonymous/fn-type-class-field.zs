class Box {
  public transform: (x: i32) => i32;
  public val: i32;
  constructor(v: i32, f: (x: i32) => i32) {
    this.val = v;
    this.transform = f;
  }
  apply(): i32 {
    return this.transform(this.val);
  }
}

function main(): i32 {
  let b = new Box(5, (x: i32): i32 => { return x * 2; });
  return b.apply();
}
