import { writeFileSync, statSync, Stats, unlinkSync } from "@std/fs";

function main(): i32 {
  let path: string = "/tmp/zeus_fs_stat.txt";
  writeFileSync(path, "0123456789"); // 10 bytes

  let s: Stats = statSync(path);
  if (s.size != 10) { return 1; }
  if (!s.isFile()) { return 2; }
  if (s.isDirectory()) { return 3; }
  if (s.mtime <= 0) { return 4; } // a just-written file has a real (nonzero) mtime

  unlinkSync(path);
  return 0;
}
