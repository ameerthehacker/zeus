// A base-typed reference holding a derived instance downcasts successfully, exposing the
// derived member.
class Animal {
  public legs: i32;
  constructor(l: i32) { this.legs = l; }
}

class Dog extends Animal {
  public goodBoy: i32;
  constructor() { super(4); this.goodBoy = 42; }
}

function main(): i32 {
  let a: Animal = new Dog();  // implicit upcast
  let d: Dog = a as Dog;      // valid downcast (runtime-checked)
  return d.goodBoy;           // 42
}
