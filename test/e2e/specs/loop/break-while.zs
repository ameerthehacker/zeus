function main(): i32 {
    // Sum 0+1+2+3+4, then break when i == 5; result should be 10
    let sum: i32 = 0;
    let i: i32 = 0;
    while (i < 100) {
        if (i == 5) {
            break;
        }
        sum = sum + i;
        i = i + 1;
    }
    // sum == 10, i == 5
    if (sum != 10) {
        return 1;
    }
    if (i != 5) {
        return 2;
    }
    return 0;
}
