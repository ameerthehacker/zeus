// A return through nested try-finally runs the finallys innermost-first.
function main(): i32 {
  try {
    try {
      return 1;
    } finally {
      console.log("inner");
    }
  } finally {
    console.log("outer");
  }
}
