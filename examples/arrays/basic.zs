// Basic Array Operations
// Run with: make run file=basic (from examples/arrays directory)
// Demonstrates: array creation, push, get, set

function main(): i32 {
  // Create a dynamic array
  let numbers: i32[] = new i32[];
  
  // Add elements
  numbers.push(10);
  numbers.push(20);
  numbers.push(30);
  numbers.push(40);
  numbers.push(50);
  
  // Access elements
  let first: i32 = numbers.get(0);   // 10
  let third: i32 = numbers.get(2);   // 30
  let last: i32 = numbers.get(4);    // 50
  
  // Modify elements
  numbers.set(2, 100);  // Change 30 to 100
  
  // Sum some elements
  let sum: i32 = first + third + numbers.get(2);
  
  return sum;  // 10 + 30 + 100 = 140
}

