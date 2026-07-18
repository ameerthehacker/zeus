// continue still runs the try's finally. i=0,2 add 1 then finally +10; i=1 continues, running only
// the finally (+10). Total = 11 + 10 + 11 = 32.
function main(): i32 {
  let sum: i32 = 0;
  for (let i: i32 = 0; i < 3; i = i + 1) {
    try {
      if (i == 1) {
        continue;
      }
      sum = sum + 1;
    } finally {
      sum = sum + 10;
    }
  }
  return sum;
}
