// loop-sum: tight integer loop 1..N accumulating (i & 1023) — raw loop / ALU.
// Result stays < 2^53 so a plain JS number is exact.
"use strict";

const expected = 334946;

let acc = 0;
const n = 1000000000;
for (let i = 1; i <= n; i++) {
  acc += i & 1023;
}
const checksum = acc % 1000003;
if (checksum !== expected) {
  process.stderr.write(`MISMATCH got=${checksum} want=${expected}\n`);
  process.exit(1);
}
process.exit(0);
