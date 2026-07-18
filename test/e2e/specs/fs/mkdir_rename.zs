import { mkdirSync, writeFileSync, readFileSync, renameSync, unlinkSync, existsSync } from "@std/fs";

function main(): i32 {
  let dir: string = "/tmp/zeus_fs_e2e_dir";
  if (!existsSync(dir)) {
    mkdirSync(dir);
  }

  let a: string = "/tmp/zeus_fs_e2e_dir/a.txt";
  let b: string = "/tmp/zeus_fs_e2e_dir/b.txt";

  writeFileSync(a, "renamed content");
  renameSync(a, b);
  if (existsSync(a)) { return 1; }

  let content: string = readFileSync(b);
  if (!content.equals("renamed content")) { return 2; }

  unlinkSync(b);
  if (existsSync(b)) { return 3; }

  return 0;
}
