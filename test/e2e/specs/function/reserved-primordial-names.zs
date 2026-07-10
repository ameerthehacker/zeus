// Regression: user functions may share names with primordial method parameters
// (count, value, index, ...). Function names live in their own codegen namespace and
// must not collide with parameter names.
function count(n: i32): i32 {
  return n + 1;
}

function value(n: i32): i32 {
  return n * 2;
}

function index(n: i32): i32 {
  return n - 1;
}

function main(): i32 {
  if (count(5) != 6) {
    return 1;
  }
  if (value(5) != 10) {
    return 2;
  }
  if (index(5) != 4) {
    return 3;
  }
  return 0;
}
