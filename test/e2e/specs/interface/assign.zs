interface HasValue {
  value: i32;
}

class Box {
  public value: i32;

  constructor(v: i32) {
    this.value = v;
  }
}

function take(hv: HasValue): i32 {
  return 0;
}

function main(): i32 {
  let b: Box = new Box(42);
  // Box conforms to HasValue, so it may be assigned to / passed as a HasValue.
  let hv: HasValue = b;

  return take(hv) + b.value;
}
