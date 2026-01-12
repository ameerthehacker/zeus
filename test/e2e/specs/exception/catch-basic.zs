function throwError(): void {
  throw new Error("Error", "Test error");
}

function main(): i32 {
  try {
    throwError();
  } catch (e: Error) {
    return 42;
  }
  return 0;
}
