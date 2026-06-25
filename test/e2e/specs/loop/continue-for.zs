function main(): i32 {
    // Sum even numbers 0..9: 0+2+4+6+8 = 20
    // continue must skip the accumulation but still run i++ (update block)
    let sum: i32 = 0;
    for (let i: i32 = 0; i < 10; i++) {
        if (i % 2 != 0) {
            continue;
        }
        sum = sum + i;
    }
    if (sum != 20) {
        return 1;
    }
    return 0;
}
