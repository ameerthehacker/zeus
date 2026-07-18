// The inner try has no matching clause, so its finally runs and the exception rethrows to the
// outer catch.
function main(): i32 {
  try {
    try {
      throw new Error("Error", "boom");
    } finally {
      console.log("finally");
    }
  } catch (e: Error) {
    return 0;
  }
  return 9;
}
