// Through a base-typed reference, the override is reached by dynamic dispatch, and the
// override's super.sound() is non-virtual — it calls the base directly rather than
// recursing back into the override.
class Animal {
  public sound(): i32 { return 10; }
}
class Dog extends Animal {
  public sound(): i32 { return super.sound() + 32; }
}
function main(): i32 {
  let a: Animal = new Dog();
  return a.sound(); // Dog.sound (virtual) -> super.sound() (non-virtual) + 32 = 42
}
