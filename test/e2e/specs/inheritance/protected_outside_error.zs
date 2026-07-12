class Base {
    protected value: i32;
    constructor(v: i32) { this.value = v; }
}

function main(): i32 {
    let b: Base = new Base(42);
    return b.value;   // error: protected member accessed from outside the hierarchy
}
