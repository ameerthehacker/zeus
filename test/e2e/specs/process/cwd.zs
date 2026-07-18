function main(): i32 {
  let dir: string = process.cwd();
  if (dir.length > 0) {
    return 0;
  }
  return 1;
}
