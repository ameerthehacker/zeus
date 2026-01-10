function main(): i8 {
  let a: string = "apple";
  let b: string = "banana";
  let c: string = "apple";

  // a < b should return -1
  let cmp1: i8 = a.compare(b);
  if (cmp1 != -1) {
    return 1;
  }

  // b > a should return 1
  let cmp2: i8 = b.compare(a);
  if (cmp2 != 1) {
    return 2;
  }

  // a == c should return 0
  let cmp3: i8 = a.compare(c);
  if (cmp3 != 0) {
    return 3;
  }

  // Success
  return 0;
}
