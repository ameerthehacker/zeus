import { mkdirSync, writeFileSync, readdirSync, unlinkSync, existsSync } from "@std/fs";

function main(): i32 {
  let dir: string = "/tmp/zeus_fs_readdir";
  if (!existsSync(dir)) {
    mkdirSync(dir);
  }
  writeFileSync(dir + "/one.txt", "1");
  writeFileSync(dir + "/two.txt", "2");

  let entries: string[] = readdirSync(dir);
  let count: i32 = 0;
  let i: i32 = 0;
  while (i < entries.length) {
    let name: string = entries[i];
    if (name.equals("one.txt") || name.equals("two.txt")) {
      count = count + 1;
    }
    i = i + 1;
  }

  unlinkSync(dir + "/one.txt");
  unlinkSync(dir + "/two.txt");

  if (count == 2) { return 0; }
  return 1;
}
