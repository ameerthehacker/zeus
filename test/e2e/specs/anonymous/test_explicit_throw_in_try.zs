function main(): i32 {
  try {
    throw new Error("TestError", "test message");
    return 1;
  } catch (e: Error) {
    return 0;
  }
}
