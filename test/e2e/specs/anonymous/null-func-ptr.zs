function main(): i32 {
  let f: (x: i32) => i32;
  try {
    f(1);
    return 1;
  } catch (e: Error) {
    return 0;
  }
}
