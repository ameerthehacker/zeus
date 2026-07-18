// BUG (wiki/compiler-bugs.md "You cannot print a number or boolean"): console.log accepts
// only string, and there is no number->string conversion, so this beginner program (print
// 1..5) does not compile. FizzBuzz is impossible without a hand-written u8[] itoa.
function main(): i32 {
  for (let i: i32 = 1; i <= 5; i++) {
    console.log(i);
  }
  return 0;
}
