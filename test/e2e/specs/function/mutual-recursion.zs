function isEven(n: i32): i32 {
  if (n == 0) {
    return 1;
  }
  return isOdd(n - 1);
}

function main(): i32 {
  return isEven(4);
}

function isOdd(n: i32): i32 {
  if (n == 0) {
    return 0;
  }
  return isEven(n - 1);
}
