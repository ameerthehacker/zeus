function main(): i32 {
  try {
    console.log("In try block");
  } catch (e: Error) {
    console.log("Caught exception");
    return 1;
  }
  return 0;
}
