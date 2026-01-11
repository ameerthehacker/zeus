// Test basic for loop
function main(): i32 {
    let sum: i32 = 0;
    
    // Basic for loop: sum 0 to 9
    for (let i: i32 = 0; i < 10; i++) {
        sum += i;
    }
    
    // sum = 0 + 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8 + 9 = 45
    if (sum != 45) {
        return 1;
    }
    
    return 0;
}
