// Test nested for loops
function main(): i32 {
    let sum: i32 = 0;
    
    // Nested for loops
    for (let i: i32 = 0; i < 3; i++) {
        for (let j: i32 = 0; j < 4; j++) {
            sum += 1;
        }
    }
    
    // 3 * 4 = 12 iterations
    if (sum != 12) {
        return 1;
    }
    
    // Another nested loop test: multiplication table sum
    let tableSum: i32 = 0;
    for (let i: i32 = 1; i <= 3; i++) {
        for (let j: i32 = 1; j <= 3; j++) {
            tableSum += i * j;
        }
    }
    // (1*1 + 1*2 + 1*3) + (2*1 + 2*2 + 2*3) + (3*1 + 3*2 + 3*3)
    // = 6 + 12 + 18 = 36
    if (tableSum != 36) {
        return 2;
    }
    
    return 0;
}
