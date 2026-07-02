function apply(f: () => i32): i32 {
  return f();
}

function getValue(): i32 { return 42; }

function main(): i32 {
  return apply(getValue);
}
