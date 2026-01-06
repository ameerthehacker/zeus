function fibonacci(n: i32): i32 {
  if (n <= 0) {
    return 0;
  }
  if (n == 1) {
    return 1;
  }
  
  let prev: i32 = 0;
  let curr: i32 = 1;
  let i: i32 = 2;
  
  while (i <= n) {
    let next: i32 = prev + curr;
    prev = curr;
    curr = next;
    i = i + 1;
  }
  
  return curr;
}

function main(): i32 {
  return fibonacci(10);  // Returns 55
}

