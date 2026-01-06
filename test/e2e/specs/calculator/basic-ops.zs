function add(a: i32, b: i32): i32 {
  return a + b;
}

function subtract(a: i32, b: i32): i32 {
  return a - b;
}

function multiply(a: i32, b: i32): i32 {
  return a * b;
}

function divide(a: i32, b: i32): i32 {
  if (b == 0) {
    return 0;
  }
  return a / b;
}

function main(): i32 {
  let a: i32 = 20;
  let b: i32 = 5;
  
  let sum: i32 = add(a, b);        // 25
  let diff: i32 = subtract(a, b);  // 15
  let prod: i32 = multiply(a, b);  // 100
  let quot: i32 = divide(a, b);    // 4
  
  return sum + diff + prod + quot;  // 144
}

