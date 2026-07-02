function helper(): i32 { return 1; }

function main(): i32 {
  function helper(): i32 { return 9; }
  return helper();
}
