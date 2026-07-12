class Base {
    protected static count: i32;

    protected static seed(): void {
        Base.count = 40;
    }
}

class Derived extends Base {
    public static compute(): i32 {
        Base.seed();            // protected static method, called from a subclass
        return Base.count + 2;  // protected static field, read from a subclass
    }
}

function main(): i32 {
    return Derived.compute();   // 40 + 2 => 42
}
