function abs(n: i32): i32 {
  if (n < 0) {
    return n * -1;
  }
  return n;
}

function max(a: i32, b: i32): i32 {
  if (a > b) {
    return a;
  }
  return b;
}

function min(a: i32, b: i32): i32 {
  if (a < b) {
    return a;
  }
  return b;
}

function power(base: i32, exp: i32): i32 {
  if (exp == 0) {
    return 1;
  }
  
  let result: i32 = 1;
  let i: i32 = 0;
  
  while (i < exp) {
    result = result * base;
    i = i + 1;
  }
  
  return result;
}

function main(): i32 {
  let a: i32 = abs(-42);     // 42
  let b: i32 = max(10, 20);  // 20
  let c: i32 = min(10, 20);  // 10
  let d: i32 = power(2, 8);  // 256
  
  // 42 + 20 + 10 + 256 = 328, but exit codes are mod 256, so 72
  return a + b + c + d;
}

