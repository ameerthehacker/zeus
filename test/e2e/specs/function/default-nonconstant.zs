// A non-constant default (a function call) is evaluated at the call site each time.
function base(): i32 {
    return 7;
}

function f(a: i32, b: i32 = base()): i32 {
    return a + b;
}

function main(): i32 {
    if (f(1) != 8) {
        return 1; // 1 + base()
    }
    if (f(1, 2) != 3) {
        return 2; // default not evaluated
    }
    return 0;
}
