// Fine-grained per-type boxing keeps toString exact: 9007199254740993 is an i64 literal and boxes
// into I64, so it prints exactly — a single f64-backed box would round it to ...992. The umbrella
// `Number` preserves this (the concrete box does the toString via interface dispatch).
console.log((9007199254740993).toString());
let big: Number = 9007199254740993;
console.log(big.toString());
