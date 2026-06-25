function main(): i32 {
    // Find the first multiple of 7 >= 20; should be 21
    let found: i32 = -1;
    for (let i: i32 = 20; i <= 100; i++) {
        if (i % 7 == 0) {
            found = i;
            break;
        }
    }
    if (found != 21) {
        return 1;
    }
    return 0;
}
