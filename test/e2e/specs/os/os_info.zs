import { platform, arch, hostname, tmpdir, totalmem } from "@std/os";

function main(): i32 {
  let p: string = platform();
  if (!p.equals("darwin") && !p.equals("linux")) { return 1; }

  let a: string = arch();
  if (!a.equals("arm64") && !a.equals("x64")) { return 2; }

  if (hostname().length == 0) { return 3; }
  if (tmpdir().length == 0) { return 4; }
  if (totalmem() <= 0) { return 5; }

  return 0;
}
