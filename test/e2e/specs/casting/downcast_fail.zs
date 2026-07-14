// An invalid downcast (the object is really a Cat, cast to Dog) throws ClassCastException at
// runtime, which is caught.
class Animal {
  public a: i32;
  constructor() { this.a = 1; }
}

class Dog extends Animal {
  constructor() { super(); }
}

class Cat extends Animal {
  constructor() { super(); }
}

function main(): i32 {
  let a: Animal = new Cat();  // dynamic type is Cat
  try {
    let d: Dog = a as Dog;    // Cat is not a Dog -> throws
    return d.a;               // unreachable
  } catch (e: Error) {
    return 42;                // caught -> success
  }
}
