// A method defined on the base class operates on an inherited field of a derived
// instance (whose object layout also carries extra derived fields).
class Counter {
  public count: i32;
  constructor() { this.count = 0; }
  public bump(): i32 {
    this.count = this.count + 1;
    return this.count;
  }
}
class Fancy extends Counter {
  public tag: i32;
  constructor() {
    this.count = 41; // inherited field
    this.tag = 7;    // own field
  }
}
function main(): i32 {
  let f: Fancy = new Fancy();
  return f.bump(); // base method mutates inherited field -> 42
}
