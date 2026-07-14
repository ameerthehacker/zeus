function main(): i32 {
  let a = [[1], [2], []];

  return a.get(0).get(0) + a.get(1).get(0) + a.length;
}
