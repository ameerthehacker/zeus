class Base {
    protected value: i32;
    constructor(v: i32) { this.value = v; }

    protected doubled(): i32 { return this.value * 2; }

    protected get tripled(): i32 { return this.value * 3; }
}

class Derived extends Base {
    constructor(v: i32) { super(v); }

    // Reads a protected field, calls a protected method, and reads a protected getter — all
    // through a *base-typed* reference from within a subclass. Each was rejected before
    // `protected` was respected in inheritance.
    combine(other: Base): i32 {
        return other.value + other.doubled() + other.tripled;
    }
}

function main(): i32 {
    let d: Derived = new Derived(7);
    return d.combine(d);   // 7 + 14 + 21 => 42
}
