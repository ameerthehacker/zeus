// Array Literals
// Run with: make run file=literals (from examples/arrays directory)
// Demonstrates: inline array literals, nesting, empty-from-annotation, numeric widening

function main(): i32 {
  // 1D literal — element type is inferred (u8[] here, since the values fit u8)
  let primes = [2, 3, 5, 7, 11];
  let sumPrimes: i32 = 0;
  let i: i32 = 0;
  while (i < primes.length) {
    sumPrimes = sumPrimes + primes[i];
    i = i + 1;
  }                                    // 2 + 3 + 5 + 7 + 11 = 28

  // Nested literal — the empty row is typed from its non-empty siblings
  let grid = [[1, 2], [3], []];
  let cells: i32 = grid[0][0] + grid[0][1] + grid[1][0] + grid.length;  // 1 + 2 + 3 + 3 = 9

  // Empty literal — element type comes from the annotation, then filled at runtime
  let names: string[] = [];
  names.push("zeus");
  names.push("array");                 // names.length = 2

  // Mixed numeric elements widen to a common type (f64[])
  let measures = [1, 2.5, 4];          // f64[]
  let ordered: i32 = measures.get(0) < measures.get(2) ? 1 : 0;  // 1

  return sumPrimes + cells + names.length + ordered;  // 28 + 9 + 2 + 1 = 40
}
