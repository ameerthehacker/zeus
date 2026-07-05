// Escaped var initialized from a method call — object type must be resolved
// at IR gen time to infer the method return type.
class Counter {
    public value: i32;
    constructor() { this.value = 5; }
    public get(): i32 { return this.value; }
}

function makeTracker(src: Counter): () => i32 {
    let count = src.get();
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let c: Counter = new Counter();
    let inc = makeTracker(c);
    inc();
    inc();
    let val = inc();
    return val - 8;  // 5 + 3 = 8; exit 0 on success
}
