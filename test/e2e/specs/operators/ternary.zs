// Test ternary operator: cond ? then : else
function main(): i32 {
    // Basic true branch
    let a: i32 = true ? 1 : 2;
    if (a != 1) {
        return 1;
    }

    // Basic false branch
    let b: i32 = false ? 1 : 2;
    if (b != 2) {
        return 2;
    }

    // With expressions as condition
    let x: i32 = 5;
    let c: i32 = x > 3 ? 10 : 20;
    if (c != 10) {
        return 3;
    }

    let d: i32 = x < 3 ? 10 : 20;
    if (d != 20) {
        return 4;
    }

    // With expressions in branches
    let e: i32 = true ? x * 2 : x + 1;
    if (e != 10) {
        return 5;
    }

    // Nested ternary (right-associative): true ? 1 : false ? 2 : 3 == 1
    let f: i32 = true ? 1 : false ? 2 : 3;
    if (f != 1) {
        return 6;
    }

    // Nested in false branch: false ? 1 : true ? 2 : 3 == 2
    let g: i32 = false ? 1 : true ? 2 : 3;
    if (g != 2) {
        return 7;
    }

    // Used inline in expression
    let h: i32 = (x > 0 ? x : -x) + 1;
    if (h != 6) {
        return 8;
    }

    // Ternary with comparison result
    let y: i32 = 0;
    let i2: i32 = y == 0 ? 99 : 0;
    if (i2 != 99) {
        return 9;
    }

    return 0;
}
