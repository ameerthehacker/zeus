function double(x: i32): i32 {
    return x * 2;
}

function apply(f: (x: i32) => i32, n: i32): i32 {
    return f(n);
}

function main(): i32 {
    let fn: (x: i32) => i32 = double;
    let result: i32 = apply(fn, 5);
    if (result != 10) {
        return 1;
    }
    return 0;
}
