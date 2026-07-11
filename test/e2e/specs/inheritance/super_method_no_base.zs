// super.method() is only valid when the class has a base class.
class Solo {
  public go(): i32 {
    return super.foo(); // Solo has no base class -> compile error
  }
}
function main(): i32 {
  let s: Solo = new Solo();
  return s.go();
}
