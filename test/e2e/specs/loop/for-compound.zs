// Test for loop with compound operators
function main(): i32 {
    let sum: i32 = 0;
    
    // For loop with += in update
    for (let i: i32 = 0; i < 20; i += 2) {
        sum += i;
    }
    // sum = 0 + 2 + 4 + 6 + 8 + 10 + 12 + 14 + 16 + 18 = 90
    if (sum != 90) {
        return 1;
    }
    
    // For loop with -= in update
    let product: i32 = 1;
    for (let i: i32 = 5; i > 0; i -= 1) {
        product *= i;
    }
    // product = 5 * 4 * 3 * 2 * 1 = 120
    if (product != 120) {
        return 2;
    }
    
    // For loop with *= in update
    let powerSum: i32 = 0;
    for (let i: i32 = 1; i <= 16; i *= 2) {
        powerSum += i;
    }
    // powerSum = 1 + 2 + 4 + 8 + 16 = 31
    if (powerSum != 31) {
        return 3;
    }
    
    return 0;
}
