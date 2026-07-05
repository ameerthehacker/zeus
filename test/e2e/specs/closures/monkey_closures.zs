// Monkey test — exercises many closure patterns in one program.
// Returns 0 only when every assertion passes; each failure returns a distinct
// non-zero exit code so failures can be pinpointed.

// 1. Mutable counter (capture-by-reference) ─────────────────────────────────
function makeCounter(): () => i32 {
    let count: i32 = 0;
    return (): i32 => {
        count += 1;
        return count;
    };
}

// 2. Adder factory (read-only capture, independent instances) ─────────────────
function makeAdder(x: i32): (y: i32) => i32 {
    return (y: i32): i32 => { return x + y; };
}

// 3. Two closures sharing the same ref cell ──────────────────────────────────
function testSharedCell(): boolean {
    let val: i32 = 0;
    let bump: () => i32 = (): i32 => { val += 1; return val; };
    let read: () => i32 = (): i32 => { return val; };
    bump();
    bump();
    return read() == 2;
}

// 4. Closure with a boolean flag (once-only semantics) ───────────────────────
function makeOnce(): () => i32 {
    let fired: boolean = false;
    let stored: i32 = 0;
    return (): i32 => {
        if (!fired) {
            fired = true;
            stored = 99;
        }
        return stored;
    };
}

// 5. Higher-order: apply a closure twice ────────────────────────────────────
function applyTwice(f: (n: i32) => i32, n: i32): i32 {
    return f(f(n));
}

// 6. Three-level nested closure capturing grandparent variable ───────────────
function outerFn(a: i32): () => () => i32 {
    let b: i32 = a + 10;
    return (): () => i32 => {
        return (): i32 => { return a + b; };
    };
}

// 7. Closure captures a class object and mutates its field ───────────────────
class Box {
    public val: i32;
    constructor() { this.val = 0; }
}

function makeBoxMutator(): () => i32 {
    let box: Box = new Box();
    box.val = 5;
    return (): i32 => {
        box.val += 1;
        return box.val;
    };
}

// 8. Class method returns a closure capturing this ───────────────────────────
class Scaler {
    public factor: i32;
    constructor() { this.factor = 4; }
    public makeScaler(): (x: i32) => i32 {
        let f: i32 = this.factor;
        return (x: i32): i32 => { return x * f; };
    }
}

// 9. Reference semantics: outer mutation visible inside ──────────────────────
function testOuterVisible(): boolean {
    let x: i32 = 1;
    let get: () => i32 = (): i32 => { return x; };
    x = 77;
    return get() == 77;
}

// 10. Reference semantics: inner mutation visible outside ────────────────────
function testInnerVisible(): boolean {
    let x: i32 = 0;
    function bump(): i32 { x += 10; return x; }
    bump();
    return x == 10;
}

// 11. Named nested function as a closure ─────────────────────────────────────
function makeAccumulator(start: i32): () => i32 {
    let total: i32 = start;
    function add(): i32 { total += 5; return total; }
    return add;
}

// 12. Fat-arrow closure capturing a function parameter ───────────────────────
function makeOffset(delta: i32): (n: i32) => i32 {
    return (n: i32): i32 => { return n + delta; };
}

// 13. Closure passed as a callback argument ───────────────────────────────────
function applyToTen(f: (n: i32) => i32): i32 {
    return f(10);
}

// 14. Closure returned after outer scope exits (escaped closure) ─────────────
function escapedCounter(): () => i32 {
    let n: i32 = 0;
    return (): i32 => { n += 1; return n; };
}

// 15. Deeply chained mutator: two independent counters stay independent ───────
function testIndependence(): boolean {
    let c1: () => i32 = makeCounter();
    let c2: () => i32 = makeCounter();
    c1(); c1(); c1();   // c1 at 3
    c2();               // c2 at 1
    return c1() == 4 && c2() == 2;
}

function main(): i32 {
    // 1. Counter
    let c: () => i32 = makeCounter();
    if (c() != 1) { return 1; }
    if (c() != 2) { return 2; }
    if (c() != 3) { return 3; }

    // 2. Independent adder instances
    let add5: (y: i32) => i32 = makeAdder(5);
    let add10: (y: i32) => i32 = makeAdder(10);
    if (add5(3) != 8)   { return 4; }
    if (add10(3) != 13) { return 5; }
    if (add5(0) != 5)   { return 6; }

    // 3. Shared ref cell
    if (!testSharedCell()) { return 7; }

    // 4. Once — first call stores value, subsequent calls return same value
    let once: () => i32 = makeOnce();
    if (once() != 99) { return 8; }
    if (once() != 99) { return 9; }

    // 5. Higher-order: apply add5 twice to 10 → 20
    if (applyTwice(add5, 10) != 20) { return 10; }

    // 6. Grandparent capture: a=5, b=15 → a+b=20
    let mid: () => () => i32 = outerFn(5);
    let inner: () => i32 = mid();
    if (inner() != 20) { return 11; }

    // 7. Box field mutation through closure
    let boxFn: () => i32 = makeBoxMutator();
    if (boxFn() != 6) { return 12; }
    if (boxFn() != 7) { return 13; }

    // 8. Class method closure
    let s: Scaler = new Scaler();
    let scale: (x: i32) => i32 = s.makeScaler();
    if (scale(5) != 20) { return 14; }
    if (scale(0) != 0)  { return 15; }

    // 9. Outer mutation visible inside
    if (!testOuterVisible()) { return 16; }

    // 10. Inner mutation visible outside
    if (!testInnerVisible()) { return 17; }

    // 11. Named function accumulator
    let acc: () => i32 = makeAccumulator(0);
    if (acc() != 5)  { return 18; }
    if (acc() != 10) { return 19; }

    // 12. Fat-arrow with captured parameter
    let plus3: (n: i32) => i32 = makeOffset(3);
    if (plus3(10) != 13) { return 20; }
    if (plus3(0)  != 3)  { return 21; }

    // 13. Callback
    let addSeven: (n: i32) => i32 = makeAdder(7);
    if (applyToTen(addSeven) != 17) { return 22; }

    // 14. Escaped counter
    let ec: () => i32 = escapedCounter();
    if (ec() != 1) { return 23; }
    if (ec() != 2) { return 24; }
    if (ec() != 3) { return 25; }

    // 15. Independence
    if (!testIndependence()) { return 26; }

    return 0;
}
