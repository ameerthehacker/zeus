// fib-recursive: naive recursive Fibonacci — exercises function-call overhead.
// checksum = fib(40) = 102334155
"use strict";

const expected = 102334155;

function fib(n) {
  if (n < 2) return n;
  return fib(n - 1) + fib(n - 2);
}

const checksum = fib(40);
if (checksum !== expected) {
  process.stderr.write(`MISMATCH got=${checksum} want=${expected}\n`);
  process.exit(1);
}
process.exit(0);
