// An override is dispatched dynamically: through a base-typed reference to a derived
// object, the derived implementation runs.
class Animal {
  public sound(): i32 { return 0; }
}
class Dog extends Animal {
  public sound(): i32 { return 42; }
}
function main(): i32 {
  let a: Animal = new Dog();
  return a.sound(); // Dog.sound via dynamic dispatch -> 42
}
