// Arithmetic and comparisons on boxed Numbers operate on the unboxed scalar; == is value equality.
let a: Number = 3;
let b: Number = 4;

let sum: Number = a + b;       // f64 result re-boxed into Number
console.log(sum.toString());

let prod: f64 = a * b;         // stays f64
console.log(prod.toString());

let less: boolean = a < b;
console.log(less.toString());

let eq: boolean = a == b;      // value equality, not pointer identity
console.log(eq.toString());
