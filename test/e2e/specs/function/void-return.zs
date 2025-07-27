function foo(x: i32): void {
  if (x > 1) {
    return;
  }
}

function main(): i8 {
  foo(10);

  return 0;
}
