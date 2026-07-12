interface ReadOnlyId {
  readonly id: i32;
}

class Thing {
  public id: i32;
  constructor(i: i32) { this.id = i; }
}

// Assigning through a readonly interface property is a compile error.
function setId(r: ReadOnlyId): void {
  r.id = 5;
}

function main(): i32 {
  let t: Thing = new Thing(1);
  setId(t);
  return 0;
}
