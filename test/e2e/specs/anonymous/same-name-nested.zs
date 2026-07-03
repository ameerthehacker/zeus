function foo(): i32 {
  function helper(): i32 { return 3; }
  return helper();
}

function bar(): i32 {
  function helper(): i32 { return 7; }
  return helper();
}

function main(): i32 {
  return foo() + bar();
}
