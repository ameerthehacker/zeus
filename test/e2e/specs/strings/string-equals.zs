function main(): i8 {
  let a: string = "hello";
  let b: string = "hello";
  let c: string = "world";
  let d: string = "hello!";

  // a == b should be true
  if (!a.equals(b)) {
    return 1;
  }

  // a != c should be true (equals returns false)
  if (a.equals(c)) {
    return 2;
  }

  // a != d should be true (different lengths)
  if (a.equals(d)) {
    return 3;
  }

  // Success
  return 0;
}
