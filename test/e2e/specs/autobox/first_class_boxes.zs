// First-class boxed types (I8..F64). Unbox and arithmetic on the concrete type stay type-exact —
// I32 unboxes to an exact i32 (no f64), and I32 + I32 computes on i32.
let n: I32 = 5;
let x: i32 = n;
console.log(x.toString());

let a: I32 = 10;
let b: I32 = 32;
let s: I32 = a + b;
console.log(s.toString());

let big: I64 = 9007199254740993;
console.log(big.toString());
