// In a derived class the field initializers are spliced in right after super(...), so the base is
// fully constructed first: super(5) sets a=5, then the derived initializer sets b=10 → 15.
class Base {
    public a: i32;

    public constructor(a: i32) {
        this.a = a;
    }
}

class Derived extends Base {
    public b: i32 = 10;

    public constructor() {
        super(5);
    }
}

function main(): i32 {
    let d: Derived = new Derived();
    return d.a + d.b;
}
