// for...of over an array of objects binds the element type
class Point {
    public x: i32;
    constructor(x: i32) {
        this.x = x;
    }
}

function main(): i32 {
    let pts: Point[] = new Point[];
    pts.push(new Point(3));
    pts.push(new Point(4));

    let sum: i32 = 0;
    for (const p of pts) {
        sum += p.x; // 3 + 4
    }

    if (sum != 7) {
        return 1;
    }
    return 0;
}
