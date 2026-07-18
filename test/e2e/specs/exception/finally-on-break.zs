// break out of the loop still runs the try's finally. i=0,1 add 1 each then finally adds 10;
// i=2 breaks, running only the finally (+10). Total = 11 + 11 + 10 = 32.
function main(): i32 {
  let sum: i32 = 0;
  for (let i: i32 = 0; i < 5; i = i + 1) {
    try {
      if (i == 2) {
        break;
      }
      sum = sum + 1;
    } finally {
      sum = sum + 10;
    }
  }
  return sum;
}
