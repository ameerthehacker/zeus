import { writeFileSync, readFileSync, unlinkSync } from "@std/fs";

// Writes a ~256KB file and reads it back, exercising readFileSync's grow-and-read loop across
// multiple 64KB chunks (cRealloc), not a single lseek-sized read.
function main(): i32 {
  let chunk: string = "";
  let i: i32 = 0;
  while (i < 1024) {
    chunk = chunk + "x";
    i = i + 1;
  }
  // Double the 1KB chunk to 256KB.
  let big: string = chunk;
  let d: i32 = 0;
  while (d < 8) {
    big = big + big;
    d = d + 1;
  }

  let path: string = "/tmp/zeus_fs_large.txt";
  writeFileSync(path, big);
  let back: string = readFileSync(path);
  unlinkSync(path);

  if (back.length == big.length && back.equals(big)) {
    return 0;
  }
  return 1;
}
