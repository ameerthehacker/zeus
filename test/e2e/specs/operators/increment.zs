// Test increment/decrement operators: ++, --
function main(): i32 {
    let a: i32 = 5;
    
    // Test prefix increment: ++a returns new value
    let b: i32 = ++a;
    if (a != 6) {
        return 1;
    }
    if (b != 6) {
        return 2;
    }
    
    // Test postfix increment: a++ returns old value
    let c: i32 = a++;
    if (a != 7) {
        return 3;
    }
    if (c != 6) {
        return 4;
    }
    
    // Test prefix decrement: --a returns new value
    let d: i32 = --a;
    if (a != 6) {
        return 5;
    }
    if (d != 6) {
        return 6;
    }
    
    // Test postfix decrement: a-- returns old value
    let e: i32 = a--;
    if (a != 5) {
        return 7;
    }
    if (e != 6) {
        return 8;
    }
    
    // Test increment in loop
    let sum: i32 = 0;
    let i: i32 = 0;
    while (i < 5) {
        sum += i;
        i++;
    }
    // sum = 0 + 1 + 2 + 3 + 4 = 10
    if (sum != 10) {
        return 9;
    }
    
    // Test decrement in loop
    let product: i32 = 1;
    let j: i32 = 5;
    while (j > 0) {
        product *= j;
        j--;
    }
    // product = 5 * 4 * 3 * 2 * 1 = 120
    if (product != 120) {
        return 10;
    }
    
    // Test prefix in expression
    let k: i32 = 3;
    let result: i32 = ++k + 10;  // k=4, result=14
    if (result != 14) {
        return 11;
    }
    if (k != 4) {
        return 12;
    }
    
    // Test postfix in expression
    let m: i32 = 3;
    let result2: i32 = m++ + 10;  // result2=13, m=4
    if (result2 != 13) {
        return 13;
    }
    if (m != 4) {
        return 14;
    }
    
    return 0;
}
