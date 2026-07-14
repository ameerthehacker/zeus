// Downcasting to an intermediate ancestor must walk the parent_type_info chain: the object is a
// Puppy, statically an Animal, downcast to Dog. Puppy IS-A Dog through Puppy -> Dog -> Animal.
class Animal {
  public a: i32;
  constructor() { this.a = 1; }
}

class Dog extends Animal {
  public d: i32;
  constructor() { super(); this.d = 2; }
}

class Puppy extends Dog {
  public p: i32;
  constructor() { super(); this.p = 3; }
}

function main(): i32 {
  let a: Animal = new Puppy();  // upcast to Animal
  let dog: Dog = a as Dog;      // downcast to the intermediate class (needs the chain walk)
  return dog.d + 40;            // 42
}
