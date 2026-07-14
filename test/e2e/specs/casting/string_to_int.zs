// A string cannot be cast to a number — rejected at compile time.
function main(): i32 {
  let s: string = "hi";
  return s as i32;
}
