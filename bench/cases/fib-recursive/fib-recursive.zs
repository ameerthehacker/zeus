// fib-recursive: naive recursive Fibonacci — exercises function-call overhead.
// Returns 0 when checksum == fib(40) == 102334155, else 1 (self-check).

function fib(n: i32): i32 {
  if (n < 2) {
    return n;
  }
  return fib(n - 1) + fib(n - 2);
}

function main(): i32 {
  let checksum: i32 = fib(40);
  if (checksum == 102334155) {
    return 0;
  }
  return 1;
}
