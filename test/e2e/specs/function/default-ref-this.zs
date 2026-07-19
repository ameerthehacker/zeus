// A method default cannot reference `this` — it is evaluated at the call site, where `this`
// is not in scope.
class C {
    public base: i32 = 5;

    add(y: i32 = this.base): i32 {
        return this.base + y;
    }
}

function main(): i32 {
    let c: C = new C();
    return c.add();
}
