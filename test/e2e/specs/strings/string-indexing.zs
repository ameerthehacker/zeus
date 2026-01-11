function main(): i32 {
  let greeting: string = "Hello";
  
  // Access individual bytes
  // H = 72, e = 101, l = 108, l = 108, o = 111
  // Sum = 72 + 101 + 108 + 108 + 111 = 500
  let sum: i32 = 0;
  
  sum = sum + greeting[0]; // H = 72
  sum = sum + greeting[1]; // e = 101
  sum = sum + greeting[2]; // l = 108
  sum = sum + greeting[3]; // l = 108
  sum = sum + greeting[4]; // o = 111
  
  // Return 500 - 500 = 0 for success
  return sum - 500;
}
