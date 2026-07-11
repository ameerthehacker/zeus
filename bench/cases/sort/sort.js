// sort: insertion sort (O(n^2)) of an LCG-filled Int32Array.
// LCG uses Math.imul + >>>0 to reproduce the exact uint32-wraparound sequence
// used by the Go and Zeus implementations.
// checksum = positional weighted sum mod MOD.
"use strict";

const N = 20000;
const MOD = 1000003;
const expected = 861863;

const arr = new Int32Array(N);
let x = 12345 >>> 0;
for (let i = 0; i < N; i++) {
  x = (Math.imul(x, 1103515245) + 12345) >>> 0;
  arr[i] = (x >>> 16) & 0xffff;
}

for (let i = 1; i < N; i++) {
  const key = arr[i];
  let j = i - 1;
  while (j >= 0 && arr[j] > key) {
    arr[j + 1] = arr[j];
    j--;
  }
  arr[j + 1] = key;
}

let sum = 0;
for (let i = 0; i < N; i++) {
  sum = (sum + (i + 1) * arr[i]) % MOD;
}
const checksum = sum;
if (checksum !== expected) {
  process.stderr.write(`MISMATCH got=${checksum} want=${expected}\n`);
  process.exit(1);
}
process.exit(0);
