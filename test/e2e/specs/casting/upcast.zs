// An explicit upcast via `as` emits no runtime check (the pointer is already a valid base
// pointer) and reads the inherited member.
class Animal {
  public a: i32;
  constructor() { this.a = 42; }
}

class Dog extends Animal {
  constructor() { super(); }
}

function main(): i32 {
  let d: Dog = new Dog();
  let a: Animal = d as Animal;  // explicit upcast, no throw
  return a.a;                   // 42
}
