function main(): i32 {
  let fns: ((x: i32) => i32)[] = new ((x: i32) => i32)[];
  fns.push((x: i32): i32 => { return x + 1; });
  fns.push((x: i32): i32 => { return x * 2; });
  return fns[1](fns[0](3));
}
