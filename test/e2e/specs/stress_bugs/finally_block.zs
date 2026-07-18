// BUG (wiki/compiler-bugs.md "finally produces a cryptic parse error"):
// `finally` is unsupported but yields `expected ;, but found {` with no hint that finally
// is the unsupported construct.
function main(): i32 {
  try {
    return 1;
  } catch (e: Error) {
    return 2;
  } finally {
    return 3;
  }
}
