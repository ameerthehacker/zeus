// A return from inside a loop-body try runs that try's finally before leaving the function. The
// return value is snapshotted first (Java-style): i=0 sum 1->11; i=1 12->22; i=2 sum=23, `return sum`
// captures 23, then the finally runs (+10, discarded). Result: 23.
function main(): i32 {
  let sum: i32 = 0;
  for (let i: i32 = 0; i < 5; i = i + 1) {
    try {
      sum = sum + 1;
      if (i == 2) {
        return sum;
      }
    } finally {
      sum = sum + 10;
    }
  }
  return sum;
}
