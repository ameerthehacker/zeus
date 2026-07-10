// Variadic anonymous function expression and variadic (block-body) arrow function,
// each stored in a const and invoked indirectly through the functor it lowers to.
function main(): i32 {
  const f = function(...args: i32[]): i32 {
    let total: i32 = 0;
    for (let i: i32 = 0; i < args.length; i++) {
      total += args[i];
    }
    return total;
  };
  if (f(1, 2, 3, 4) != 10) {
    return 1;
  }
  if (f() != 0) {
    return 2;
  }

  const g = (...args: i32[]): i32 => {
    return args.length;
  };
  if (g(5, 6, 7) != 3) {
    return 3;
  }
  return 0;
}
