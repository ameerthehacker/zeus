// matmul: naive NxN integer matrix multiply — a memory-bound triple-nested
// loop. Matrices are stored flat (row-major, i*n+j) so every language shares
// the same layout. checksum = sum of C mod 1000003 == 434781 (self-check).

function main(): i32 {
  let n: i32 = 512;
  let size: i32 = 262144;
  let a: i32[] = new i32[262144];
  let b: i32[] = new i32[262144];
  let c: i32[] = new i32[262144];

  for (let fi: i32 = 0; fi < n; fi = fi + 1) {
    for (let fj: i32 = 0; fj < n; fj = fj + 1) {
      a[fi * n + fj] = (fi + fj) % 100;
      b[fi * n + fj] = (fi * fj) % 100;
    }
  }

  for (let i: i32 = 0; i < n; i = i + 1) {
    for (let j: i32 = 0; j < n; j = j + 1) {
      let s: i32 = 0;
      for (let k: i32 = 0; k < n; k = k + 1) {
        s = s + a[i * n + k] * b[k * n + j];
      }
      c[i * n + j] = s;
    }
  }

  let sum: i64 = 0;
  for (let idx: i32 = 0; idx < size; idx = idx + 1) {
    sum = (sum + c[idx]) % 1000003;
  }

  if (sum == 434781) {
    return 0;
  }
  return 1;
}
