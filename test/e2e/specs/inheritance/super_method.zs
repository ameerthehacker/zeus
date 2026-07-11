// super.method() calls the base implementation, letting an override extend it.
class Animal {
  public sound(): i32 { return 10; }
}
class Dog extends Animal {
  public sound(): i32 { return super.sound() + 32; }
}
function main(): i32 {
  let d: Dog = new Dog();
  return d.sound(); // 10 (base) + 32 = 42
}
