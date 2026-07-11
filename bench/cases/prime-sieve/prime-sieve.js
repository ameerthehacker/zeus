// prime-sieve: Sieve of Eratosthenes up to LIMIT, counts primes.
// Uint8Array for the sieve buffer (idiomatic fast JS). The `p <= limit/p`
// guard keeps p*p within 32-bit range.
"use strict";

const limit = 30000000;
const expected = 1857859;

const sieve = new Uint8Array(limit + 1);
let count = 0;
for (let p = 2; p <= limit; p++) {
  if (sieve[p] === 0) {
    count++;
    if (p <= Math.floor(limit / p)) {
      for (let j = p * p; j <= limit; j += p) sieve[j] = 1;
    }
  }
}
const checksum = count;
if (checksum !== expected) {
  process.stderr.write(`MISMATCH got=${checksum} want=${expected}\n`);
  process.exit(1);
}
process.exit(0);
