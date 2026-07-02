function main(): i32 {
  let fib = function fibonacci(n: i32): i32 {
    if (n <= 1) { return n; }
    return fibonacci(n - 1) + fibonacci(n - 2);
  }
  return fibonacci(8);
}
