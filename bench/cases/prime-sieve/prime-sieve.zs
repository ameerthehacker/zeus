// prime-sieve: Sieve of Eratosthenes up to LIMIT, counts primes.
// The `p <= limit / p` guard keeps p*p within 32-bit range.
// Returns 0 when count == 1857859 (primes below 30,000,000), else 1.

function main(): i32 {
  let limit: i32 = 30000000;
  let sieve: u8[] = new u8[limit + 1];
  let count: i32 = 0;

  for (let p: i32 = 2; p <= limit; p = p + 1) {
    if (sieve[p] == 0) {
      count = count + 1;
      if (p <= limit / p) {
        let j: i32 = p * p;
        while (j <= limit) {
          sieve[j] = 1;
          j = j + p;
        }
      }
    }
  }

  if (count == 1857859) {
    return 0;
  }
  return 1;
}
