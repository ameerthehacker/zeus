function main(): i32 {
  let array: i32[] = new i32[];
  array.push(10);
  array.push(20);

  // This should throw IndexOutOfBoundsException (index 5 is out of bounds for array of length 2)
  let value: i32 = array[5];

  return value;
}
