// A Number unboxes to f64 and a Bool to boolean when assigned into a primitive slot.
let n: Number = 6;
let d: f64 = n;
console.log(d.toString());

let b: Bool = false;
let raw: boolean = b;
console.log(raw.toString());
