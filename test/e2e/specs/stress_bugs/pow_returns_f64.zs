// BUG (wiki/compiler-bugs.md "** always produces f64"): a pure-integer power expression
// yields f64 and cannot be assigned to an integer without an explicit cast.
function main(): i32 {
  let n: i32 = 2 ** 10;   // error: type 'f64' is not assignable to type 'i32'
  return n - 1024;
}
