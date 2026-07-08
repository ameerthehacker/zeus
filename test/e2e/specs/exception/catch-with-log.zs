function throwError(): void {
  throw new Error("Error", "Test error");
}

function main(): i32 {
  try {
    throwError();
  } catch (e: Error) {
    console.log("Caught error: " + e.message);
    return 0;
  }
  return 1;
}
