// Name-collision regression: many methods and functions share a source name across unrelated
// classes, an inheritance override, and different function scopes. Each must resolve to ITS OWN
// compiled function — codegen resolves methods by their unique IR name through a controlled
// name->function map (vtable fill + super calls), never by fetching a possibly collision-renamed
// symbol from LLVM's global table. Every check returns a distinct non-zero exit code on failure;
// exit 0 means every same-named call dispatched to the correct function.

// 1) Same method name `tag` on three unrelated classes.
class Alpha { public tag(): i32 { return 1; } }
class Beta  { public tag(): i32 { return 2; } }
class Gamma { public tag(): i32 { return 3; } }

// A method `pick` that also collides with the nested functions below.
class Picker { public pick(): i32 { return 7; } }

// 2) Inheritance: `kind` is overridden (and calls super); `label` is inherited unchanged.
class Base {
    public kind(): i32 { return 10; }
    public label(): i32 { return 20; }
}
class Derived extends Base {
    public kind(): i32 { return super.kind() + 5; } // 15
}

// 3) A free function whose name collides with the `tag` methods.
function tag(): i32 { return 100; }

// 4) Same nested-function name `pick` in two different function scopes, distinct values.
function scopedA(): i32 {
    function pick(): i32 { return 41; }
    return pick();
}
function scopedB(): i32 {
    function pick(): i32 { return 42; }
    return pick();
}

function main(): i32 {
    let a: Alpha = new Alpha();
    let b: Beta = new Beta();
    let g: Gamma = new Gamma();
    if (a.tag() != 1) { return 1; }
    if (b.tag() != 2) { return 2; }
    if (g.tag() != 3) { return 3; }

    if (tag() != 100) { return 4; } // free function, not any method

    let d: Derived = new Derived();
    if (d.kind() != 15) { return 5; }  // override + super
    if (d.label() != 20) { return 6; } // inherited slot resolves to Base.label

    let asBase: Base = d;
    if (asBase.kind() != 15) { return 7; } // dynamic dispatch reaches the override
    let realBase: Base = new Base();
    if (realBase.kind() != 10) { return 8; } // and the base's own slot

    let p: Picker = new Picker();
    if (p.pick() != 7) { return 9; } // method `pick` vs the nested `pick`s
    if (scopedA() != 41) { return 10; }
    if (scopedB() != 42) { return 11; }
    return 0;
}
