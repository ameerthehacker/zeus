// A nested function inside a try is its own control-flow scope: its return must NOT run the enclosing
// try's finally. Here the finally runs exactly once (when work() returns), so counter is 1, not 2.
let counter: i32 = 0;

function work(): i32 {
  try {
    let f = (): i32 => { return 5; };
    return f();
  } finally {
    counter = counter + 1;
  }
}

function main(): i32 {
  let x: i32 = work();
  return counter + x - 5;
}
