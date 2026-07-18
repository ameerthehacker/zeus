// Boxed values work as conditions, in comparisons, and under unary operators; f32 toString is exact
// at native width (regression guards for the boxing code review).
let b: Bool = true;
if (b) { console.log("if-ok"); }

let n: Number = 5;
if (n > 3) { console.log("cmp-ok"); }

let c: boolean = !b;
console.log(c.toString());

let f: f32 = 0.1;
console.log(f.toString());
