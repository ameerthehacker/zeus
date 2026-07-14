// A class with initialized instance fields but no constructor: an implicit constructor runs the
// initializers, so `new Point()` yields x=3, y=4 → 7.
class Point {
    public x: i32 = 3;
    public y: i32 = 4;
}

function main(): i32 {
    let p: Point = new Point();
    return p.x + p.y;
}
