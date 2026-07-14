function main(): i32 {
  // The int element 1 widens to the common numeric type f64.
  let a = [1, 2.5];

  return a.get(0) < a.get(1) ? 1 : 0;
}
