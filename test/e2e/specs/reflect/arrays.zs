// Runtime reflection renders arrays as [e0, e1, ...] — primitive, string (quoted), bool, empty,
// object-element, and nested arrays (each element via its own runtime header).
class Point { x: i32; constructor(x: i32) { this.x = x; } }

let nums: i32[] = [10, 20, 30];
console.log(nums);
let fs: f64[] = [1.5, 2.0];
console.log(fs);
let strs: string[] = ["a", "b"];
console.log(strs);
let flags: boolean[] = [true, false];
console.log(flags);
let empty: i32[] = [];
console.log(empty);
let pts: Point[] = [new Point(1), new Point(2)];
console.log(pts);
let nested: i32[][] = [[1, 2], [3]];
console.log(nested);
