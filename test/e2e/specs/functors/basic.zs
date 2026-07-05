class AddFunctor {
  public __call__(a: i32, b: i32): i32 {
    return a + b;
  }
}

function main(): i32 {
  const add = new AddFunctor();
  return add(1, 2);
}
