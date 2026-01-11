// Test for loop with decrement
function main(): i32 {
    let sum: i32 = 0;
    
    // For loop counting down
    for (let i: i32 = 10; i > 0; i--) {
        sum += i;
    }
    
    // sum = 10 + 9 + 8 + 7 + 6 + 5 + 4 + 3 + 2 + 1 = 55
    if (sum != 55) {
        return 1;
    }
    
    return 0;
}
