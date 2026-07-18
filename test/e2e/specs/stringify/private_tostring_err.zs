// A private toString cannot satisfy Stringify; the hint says to make it public.
class C { private toString(): string { return "c"; } }
let c: C = new C();
console.log(c);
