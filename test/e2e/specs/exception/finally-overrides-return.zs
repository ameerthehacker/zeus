// A return inside the finally overrides the pending return from the try.
function main(): i32 {
  try {
    return 5;
  } finally {
    return 9;
  }
}
