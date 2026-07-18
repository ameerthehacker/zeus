// BUG (wiki/compiler-bugs.md "Template ${...} / + with a number gives a misleading error"):
// interpolating a number reports an "invalid ... binary operation" error even though the
// user typed no operator, just ${n}.
function main(): i32 {
  let n: i32 = 42;
  let s: string = `value is ${n}`;
  console.log(s);
  return 0;
}
