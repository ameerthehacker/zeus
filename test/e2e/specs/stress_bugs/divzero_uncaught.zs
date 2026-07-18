// REGRESSION (fixed): integer divide-by-zero now throws a catchable ArithmeticException (was
// silent UB that fell through to `return 1`). The catch fires and returns 2.
function main(): i32 {
  let a: i32 = 10;
  let b: i32 = 0;
  try {
    let c: i32 = a / b;
    return 1;   // unreachable now: the division throws
  } catch (e: Error) {
    return 2;   // ArithmeticException is caught here
  }
}
