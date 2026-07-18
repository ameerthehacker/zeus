// A value whose type has no toString cannot convert to string; the error hints at Stringify.
class Bare { public x: i32; public constructor() { this.x = 0; } }
let o: Bare = new Bare();
console.log(o);
