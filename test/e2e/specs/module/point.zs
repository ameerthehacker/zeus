export class Point {
    public x: i32;
    public y: i32;

    constructor(x: i32, y: i32) {
        this.x = x;
        this.y = y;
    }

    public getSum(): i32 {
        return this.x + this.y;
    }
}
