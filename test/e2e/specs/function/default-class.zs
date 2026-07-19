// Default parameters on a constructor, an instance method, and a static method.
class Calc {
    base: i32;

    constructor(base: i32 = 100) {
        this.base = base;
    }

    add(x: i32, y: i32 = 5): i32 {
        return this.base + x + y;
    }

    static make(v: i32 = 7): i32 {
        return v * 2;
    }
}

function main(): i32 {
    let c: Calc = new Calc();          // constructor default: base = 100
    if (c.add(1) != 106) {
        return 1; // 100 + 1 + 5
    }
    if (c.add(1, 2) != 103) {
        return 2; // 100 + 1 + 2
    }

    let c2: Calc = new Calc(10);       // constructor arg provided
    if (c2.add(1) != 16) {
        return 3; // 10 + 1 + 5
    }

    if (Calc.make() != 14) {
        return 4; // static default: 7 * 2
    }
    if (Calc.make(3) != 6) {
        return 5; // 3 * 2
    }
    return 0;
}
