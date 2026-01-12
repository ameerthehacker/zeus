function throwError(): void {
  throw new Error("Error", "Test error");
}

function main(): i32 {
  try {
    throwError();
  } catch (e: Error) {
    log("Caught error: " + e.message);
    return 0;
  }
  return 1;
}
