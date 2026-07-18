// FIXED: `finally` is supported. It runs on every exit path, and a return in the finally overrides
// the pending returns from the try and catch — so this program returns 3.
function main(): i32 {
  try {
    return 1;
  } catch (e: Error) {
    return 2;
  } finally {
    return 3;
  }
}
