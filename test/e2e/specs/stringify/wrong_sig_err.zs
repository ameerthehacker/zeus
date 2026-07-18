// A toString with the wrong return type doesn't satisfy Stringify; the hint says fix the signature.
class C { public toString(): i32 { return 0; } }
let c: C = new C();
console.log(c);
