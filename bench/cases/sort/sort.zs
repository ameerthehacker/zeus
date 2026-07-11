// sort: insertion sort (O(n^2)) of an LCG-filled i32 array — array get/set + compares.
// The LCG runs in u32 (wraparound), reproducing the exact same uint32 sequence as the
// Go/JS implementations. checksum = positional weighted sum mod 1000003 == 861863.

function main(): i32 {
  let n: i32 = 20000;
  let arr: i32[] = new i32[20000];

  let x: u32 = 12345;
  for (let fi: i32 = 0; fi < n; fi = fi + 1) {
    x = x * 1103515245 + 12345;
    arr[fi] = (x >> 16) & 65535;
  }

  for (let si: i32 = 1; si < n; si = si + 1) {
    let key: i32 = arr[si];
    let j: i32 = si - 1;
    while (j >= 0 && arr[j] > key) {
      arr[j + 1] = arr[j];
      j = j - 1;
    }
    arr[j + 1] = key;
  }

  let sum: i64 = 0;
  for (let ci: i32 = 0; ci < n; ci = ci + 1) {
    sum = (sum + (ci + 1) * arr[ci]) % 1000003;
  }

  if (sum == 861863) {
    return 0;
  }
  return 1;
}
