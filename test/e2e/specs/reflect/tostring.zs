// x.toString() is callable on ANY value — reflection is a language feature anyone can use. A class
// with no toString reflects its structural debug string; a class that defines one overrides that;
// arrays and primitives stringify too.
class Bare { x: i32; constructor() { this.x = 42; } }
class Named { n: string; constructor() { this.n = "hi"; } toString(): string { return "N(" + this.n + ")"; } }
console.log((new Bare()).toString());
console.log((new Named()).toString());
console.log([1, 2, 3].toString());
console.log((5).toString());
console.log(true.toString());
