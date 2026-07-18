function main(): i32 {
  let r: i32 = 0;
  try {
    throw new Error("Error", "boom");
  } catch (e: Error) {
    r = 1;
  } finally {
    r = r + 10;
  }
  return r;
}
