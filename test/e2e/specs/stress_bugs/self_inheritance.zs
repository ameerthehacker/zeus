// REGRESSION (fixed): an inheritance cycle (`class A extends A`) is now rejected at compile time
// with a "circular inheritance" error, instead of compiling and segfaulting at construction.
class A extends A {
  public x: i32;
}
function main(): i32 {
  return 0;
}
