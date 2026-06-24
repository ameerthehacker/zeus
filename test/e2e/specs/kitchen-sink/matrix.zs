class Matrix {
    public data: i32[][];
    public rows: i32;
    public cols: i32;

    constructor(rows: i32, cols: i32) {
        this.rows = rows;
        this.cols = cols;
        this.data = new i32[][];
        for (let i: i32 = 0; i < rows; i++) {
            this.data.push(new i32[cols]);
        }
    }

    public set(r: i32, c: i32, v: i32): void {
        this.data[r][c] = v;
    }

    public get(r: i32, c: i32): i32 {
        return this.data[r][c];
    }

    public multiply(other: Matrix): Matrix {
        let result: Matrix = new Matrix(this.rows, other.cols);
        for (let i: i32 = 0; i < this.rows; i++) {
            for (let j: i32 = 0; j < other.cols; j++) {
                let sum: i32 = 0;
                for (let k: i32 = 0; k < this.cols; k++) {
                    sum += this.get(i, k) * other.get(k, j);
                }
                result.set(i, j, sum);
            }
        }
        return result;
    }

    public trace(): i32 {
        let sum: i32 = 0;
        let n: i32 = this.rows;
        if (this.cols < n) {
            n = this.cols;
        }
        for (let i: i32 = 0; i < n; i++) {
            sum += this.get(i, i);
        }
        return sum;
    }
}

function main(): i32 {
    let identity: Matrix = new Matrix(2, 2);
    identity.set(0, 0, 1);
    identity.set(1, 1, 1);

    let a: Matrix = new Matrix(2, 2);
    a.set(0, 0, 1);
    a.set(0, 1, 2);
    a.set(1, 0, 3);
    a.set(1, 1, 4);

    let r1: Matrix = identity.multiply(a);
    if (r1.get(0, 0) != 1) { return 1; }
    if (r1.get(0, 1) != 2) { return 2; }
    if (r1.get(1, 0) != 3) { return 3; }
    if (r1.get(1, 1) != 4) { return 4; }

    let b: Matrix = new Matrix(2, 2);
    b.set(0, 0, 5);
    b.set(0, 1, 6);
    b.set(1, 0, 7);
    b.set(1, 1, 8);

    let r2: Matrix = a.multiply(b);
    if (r2.get(0, 0) != 19) { return 5; }
    if (r2.get(0, 1) != 22) { return 6; }
    if (r2.get(1, 0) != 43) { return 7; }
    if (r2.get(1, 1) != 50) { return 8; }

    if (r2.trace() != 69) { return 9; }

    return 0;
}
