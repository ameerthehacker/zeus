// A try/finally with no catch runs the finally on normal completion.
function main(): i32 {
  let r: i32 = 0;
  try {
    r = 5;
  } finally {
    r = r + 1;
  }
  return r;
}
