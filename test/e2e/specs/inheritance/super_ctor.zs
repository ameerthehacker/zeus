// A derived constructor chains to the base constructor with super(...); the base sets its
// own field, the derived sets its own.
class Animal {
  public legs: i32;
  constructor(legs: i32) { this.legs = legs; }
}
class Dog extends Animal {
  public tail: i32;
  constructor(legs: i32, tail: i32) {
    super(legs);
    this.tail = tail;
  }
}
function main(): i32 {
  let d: Dog = new Dog(40, 2);
  return d.legs + d.tail; // 42
}
