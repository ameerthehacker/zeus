interface Counter {
  count: i32;
}

class Tally {
  public count: i32;
  constructor() { this.count = 0; }
}

// Writes a property through the interface value; the change is visible on the concrete object.
function bump(c: Counter): void {
  c.count = 42;
}

function main(): i32 {
  let t: Tally = new Tally();
  bump(t);
  return t.count;
}
