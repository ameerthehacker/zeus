// A catch body that itself throws lands in the finally's outer handler: finally runs, then the new
// exception rethrows to the outer catch.
function main(): i32 {
  try {
    try {
      throw new Error("Error", "first");
    } catch (e: Error) {
      throw new Error("Error", "second");
    } finally {
      console.log("cleanup");
    }
  } catch (e: Error) {
    return 0;
  }
  return 9;
}
