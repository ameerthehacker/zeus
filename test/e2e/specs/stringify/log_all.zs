// Implicit T -> string conversion lets console.log accept any Stringify value (all primitives + Number).
console.log(5);
console.log(true);
console.log(3.14);
console.log(9007199254740993);
let n: Number = 7;
console.log(n);
