function main(): u32 {
  let array: u8[] = new u8[10];

  array.set(0, 10);
  array.set(1, 20);
  array.set(2, 30);

  // Test 1D array indexing: array[0] + array[1] + array[2]
  return array[0] + array[1] + array[2];
}

