// loop-sum: tight integer loop 1..N accumulating (i & 1023) — raw loop / ALU.
// Accumulator is i64; final checksum reduced mod 1000003 == 334946 (self-check).

function main(): i32 {
  let acc: i64 = 0;
  for (let i: i64 = 1; i <= 1000000000; i = i + 1) {
    acc = acc + (i & 1023);
  }
  let checksum: i64 = acc % 1000003;
  if (checksum == 334946) {
    return 0;
  }
  return 1;
}
