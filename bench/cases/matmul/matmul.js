// matmul: naive NxN integer matrix multiply — a memory-bound triple-nested
// loop. Matrices are stored flat (row-major, i*N+j) in Int32Array so every
// language shares the same layout. checksum = sum of C mod MOD.
"use strict";

const N = 512;
const MOD = 1000003;
const expected = 434781;

const a = new Int32Array(N * N);
const b = new Int32Array(N * N);
const c = new Int32Array(N * N);
for (let i = 0; i < N; i++) {
  for (let j = 0; j < N; j++) {
    a[i * N + j] = (i + j) % 100;
    b[i * N + j] = (i * j) % 100;
  }
}

for (let i = 0; i < N; i++) {
  for (let j = 0; j < N; j++) {
    let s = 0;
    for (let k = 0; k < N; k++) {
      s += a[i * N + k] * b[k * N + j];
    }
    c[i * N + j] = s;
  }
}

let sum = 0;
for (let idx = 0; idx < N * N; idx++) {
  sum = (sum + c[idx]) % MOD;
}
const checksum = sum;
if (checksum !== expected) {
  process.stderr.write(`MISMATCH got=${checksum} want=${expected}\n`);
  process.exit(1);
}
process.exit(0);
