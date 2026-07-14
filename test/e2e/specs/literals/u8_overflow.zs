// A literal that doesn't fit the target integer type is rejected at compile time.
function main(): i32 {
  let x: u8 = 300;  // 300 > 255
  return x as i32;
}
