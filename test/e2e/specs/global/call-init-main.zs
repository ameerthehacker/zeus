function seed(): i32 {
  return 7;
}

// Module-level const initialized by a function call runs in the module init at startup.
const a: i32 = seed();

function main(): i32 {
  return a;
}
