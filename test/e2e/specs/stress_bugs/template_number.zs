// FIXED: template interpolation of a number now works — ${n} converts via universal toString.
function main(): i32 {
  let n: i32 = 42;
  let s: string = `value is ${n}`;
  console.log(s);
  return 0;
}
