// A user class implements Stringify by declaring toString(); a string is itself Stringify.
class Point {
    public x: i32;
    public constructor(x: i32) { this.x = x; }
    public toString(): string { return "Point(" + this.x + ")"; }
}
let p: Point = new Point(9);
console.log(p);

function show(s: Stringify): string { return s.toString(); }
console.log(show("hi"));
console.log(show(42));
