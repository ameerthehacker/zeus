// Multiple trailing default parameters are filled left-to-right for the omitted ones.
function f(a: i32, b: i32 = 2, c: i32 = 3): i32 {
    return a + b + c;
}

function main(): i32 {
    if (f(1) != 6) {
        return 1; // 1 + 2 + 3
    }
    if (f(1, 10) != 14) {
        return 2; // 1 + 10 + 3
    }
    if (f(1, 10, 20) != 31) {
        return 3; // 1 + 10 + 20
    }
    return 0;
}
