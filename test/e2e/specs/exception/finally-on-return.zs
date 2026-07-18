// A return inside try runs the finally before actually returning.
function main(): i32 {
  try {
    return 5;
  } finally {
    console.log("done");
  }
}
