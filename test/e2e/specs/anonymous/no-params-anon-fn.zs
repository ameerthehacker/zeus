function main(): i32 {
  let getValue = function(): i32 { return 42; };
  let getZero = (): i32 => { return 0; };
  return getValue() + getZero() + 0;
}
