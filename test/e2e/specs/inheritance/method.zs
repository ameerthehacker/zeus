// A derived class inherits its base class's methods.
class Base {
  public value(): i32 { return 42; }
}
class Derived extends Base { }
function main(): i32 {
  let d: Derived = new Derived();
  return d.value(); // inherited -> 42
}
