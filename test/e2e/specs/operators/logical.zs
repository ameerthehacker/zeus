// Test logical operators: &&, ||, !
function main(): i32 {
    // Test && (logical AND)
    if (!(true && true)) {
        return 1;
    }
    if (true && false) {
        return 2;
    }
    if (false && true) {
        return 3;
    }
    if (false && false) {
        return 4;
    }
    
    // Test || (logical OR)
    if (!(true || true)) {
        return 5;
    }
    if (!(true || false)) {
        return 6;
    }
    if (!(false || true)) {
        return 7;
    }
    if (false || false) {
        return 8;
    }
    
    // Test ! (logical NOT)
    if (!true) {
        return 9;
    }
    if (!!false) {
        return 10;
    }
    
    // Test complex expressions
    let a: boolean = true;
    let b: boolean = false;
    
    // (true && true) || false = true
    if (!((a && a) || b)) {
        return 11;
    }
    
    // (false && true) || true = true
    if (!((b && a) || a)) {
        return 12;
    }
    
    // !(true && false) = true
    if (!(!(a && b))) {
        return 13;
    }
    
    // Comparison results with logical operators
    let x: i32 = 5;
    let y: i32 = 10;
    
    // (5 < 10) && (10 > 5) = true
    if (!((x < y) && (y > x))) {
        return 14;
    }
    
    // (5 > 10) || (10 > 5) = true
    if (!((x > y) || (y > x))) {
        return 15;
    }
    
    return 0;
}
