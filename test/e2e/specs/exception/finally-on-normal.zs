function main(): i32 {
  let r: i32 = 0;
  try {
    r = 1;
  } finally {
    r = r + 10;
  }
  return r;
}
