// REGRESSION (fixed): a ternary now unifies both branches to the common numeric type instead
// of fixing it to the first branch, so `flag ? 1 : 2.0` is f64 (was: "f64 not assignable to i32").
function main(): i32 {
  let flag: boolean = true;
  let x: f64 = flag ? 1 : 2.0;   // x == 1.0
  let y: f64 = !flag ? 1 : 2.0;  // y == 2.0
  if (x > 0.5 && x < 1.5 && y > 1.5 && y < 2.5) { return 0; }
  return 1;
}
