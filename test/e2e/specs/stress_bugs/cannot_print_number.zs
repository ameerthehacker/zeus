// FIXED: console.log is variadic and converts any value via universal toString, so printing
// numbers works — this beginner program (print 1..5) compiles and runs.
function main(): i32 {
  for (let i: i32 = 1; i <= 5; i++) {
    console.log(i);
  }
  return 0;
}
