function main(): u32 {
  let array: u8[] = new u8[10];

  // Test 1D array assignment using bracket notation
  array[0] = 10;
  array[1] = 20;
  array[2] = 30;

  // Verify values were set correctly: array[0] + array[1] + array[2]
  return array[0] + array[1] + array[2];
}


