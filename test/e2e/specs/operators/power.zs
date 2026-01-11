// Test power operator
function main(): i32 {
    // Integer power: 2 ** 3 = 8
    let a: i32 = 2;
    let b: i32 = 3;
    if (a ** b != 8) {
        return 1;
    }
    
    // Integer power: 3 ** 4 = 81
    let c: i32 = 3;
    let d: i32 = 4;
    if (c ** d != 81) {
        return 2;
    }
    
    // Power of 0: x ** 0 = 1
    let e: i32 = 5;
    let f: i32 = 0;
    if (e ** f != 1) {
        return 3;
    }
    
    // Power with 1: x ** 1 = x
    let g: i32 = 7;
    let h: i32 = 1;
    if (g ** h != 7) {
        return 4;
    }
    
    // 10 ** 2 = 100
    let i: i32 = 10;
    let j: i32 = 2;
    if (i ** j != 100) {
        return 5;
    }
    
    return 0;
}
