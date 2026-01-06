function sumFibonacci(n: i32): i32 {
  let sum: i32 = 0;
  let prev: i32 = 0;
  let curr: i32 = 1;
  
  if (n >= 1) {
    sum = sum + prev;  // Add F(0)
  }
  
  let i: i32 = 1;
  while (i < n) {
    sum = sum + curr;
    let next: i32 = prev + curr;
    prev = curr;
    curr = next;
    i = i + 1;
  }
  
  return sum;
}

function main(): i32 {
  return sumFibonacci(10);  // Sum of first 10: 0+1+1+2+3+5+8+13+21+34 = 88
}

