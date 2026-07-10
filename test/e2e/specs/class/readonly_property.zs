class Point {
    public readonly x: i32;
    public readonly y: i32;

    constructor(x: i32, y: i32) {
        this.x = x;
        this.y = y;
    }

    getX(): i32 {
        return this.x;
    }
}

function main(): i32 {
    let p = new Point(3, 4);
    if (p.x != 3) { return 1; }
    if (p.y != 4) { return 2; }
    if (p.getX() != 3) { return 3; }
    return 0;
}
