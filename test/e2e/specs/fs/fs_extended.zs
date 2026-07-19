import { writeFileSync, readFileSync, appendFileSync, copyFileSync, existsSync, mkdirpSync, rmSync, statSync, lstatSync, realpathSync, chmodSync, truncateSync, readdirTypesSync, Dirent, Stats } from "@std/fs";

// Exercises the extended @std/fs surface. Distinct sentinels; 0 = all pass.
function main(): i32 {
  let base: string = "/tmp/zeus_fs_extended_e2e";
  if (existsSync(base)) { rmSync(base, true); }

  mkdirpSync(base + "/a/b");
  if (!existsSync(base + "/a/b")) { return 1; }

  writeFileSync(base + "/f.txt", "hello");
  appendFileSync(base + "/f.txt", " world");
  if (!readFileSync(base + "/f.txt").equals("hello world")) { return 2; }

  copyFileSync(base + "/f.txt", base + "/g.txt");
  truncateSync(base + "/g.txt", 5 as i64);
  if (!readFileSync(base + "/g.txt").equals("hello")) { return 3; }

  chmodSync(base + "/g.txt", 420);
  let st: Stats = statSync(base + "/g.txt");
  if (!st.isFile()) { return 4; }
  if (st.size != 5) { return 5; }

  let ls: Stats = lstatSync(base + "/a");
  if (!ls.isDirectory()) { return 6; }
  if (realpathSync(base + "/a/b/..").length == 0) { return 7; }

  let entries: Dirent[] = readdirTypesSync(base);
  let files: i32 = 0;
  let dirs: i32 = 0;
  let i: i32 = 0;
  while (i < entries.length) {
    if (entries[i].isFile()) { files = files + 1; }
    if (entries[i].isDirectory()) { dirs = dirs + 1; }
    i = i + 1;
  }
  if (files != 2) { return 8; }
  if (dirs != 1) { return 9; }

  rmSync(base, true);
  if (existsSync(base)) { return 10; }

  return 0;
}
