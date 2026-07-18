import { writeFileSync, readFileSync, existsSync, unlinkSync } from "@std/fs";

function main(): i32 {
  let path: string = "/tmp/zeus_fs_e2e_rw.txt";
  let content: string = "hello from zeus fs\nline two";

  writeFileSync(path, content);
  if (!existsSync(path)) { return 1; }

  let readBack: string = readFileSync(path);
  if (!readBack.equals(content)) { return 2; }

  unlinkSync(path);
  if (existsSync(path)) { return 3; }

  return 0;
}
