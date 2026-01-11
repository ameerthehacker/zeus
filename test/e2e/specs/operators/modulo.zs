// Test modulo operator
function main(): i32 {
    let a: i32 = 17;
    let b: i32 = 5;
    
    // 17 % 5 = 2
    if (a % b != 2) {
        return 1;
    }
    
    // Test with different values
    let c: i32 = 20;
    let d: i32 = 7;
    // 20 % 7 = 6
    if (c % d != 6) {
        return 2;
    }
    
    // Test with zero remainder
    let e: i32 = 15;
    let f: i32 = 5;
    // 15 % 5 = 0
    if (e % f != 0) {
        return 3;
    }
    
    // Test negative modulo (signed)
    let g: i32 = -17;
    let h: i32 = 5;
    // -17 % 5 = -2 (sign follows dividend in LLVM)
    if (g % h != -2) {
        return 4;
    }
    
    return 0;
}
