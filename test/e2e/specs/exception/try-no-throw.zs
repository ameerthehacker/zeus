function main(): i32 {
  try {
    log("In try block");
  } catch (e: Error) {
    log("Caught exception");
    return 1;
  }
  return 0;
}
