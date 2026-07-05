class Adder {
    public base: i32;
    constructor(b: i32) { this.base = b; }
    public makeAdder(extra: i32): () => i32 {
        function compute(): i32 {
            return this.base + extra;
        }
        return compute;
    }
}

function main(): i32 {
    let a: Adder = new Adder(30);
    let f: () => i32 = a.makeAdder(20);
    return f();
}
