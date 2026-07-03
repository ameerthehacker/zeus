class Box {
  public val: i32;
  constructor(v: i32) { this.val = v; }
}

function readVal(b: Box): i32 {
  return b.val;
}

function main(): i32 {
  try {
    let b: Box;
    let v: i32 = readVal(b);
    return 1;
  } catch (e: Error) {
    return 0;
  }
}
