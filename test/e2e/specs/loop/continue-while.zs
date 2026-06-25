function main(): i32 {
    // Sum only odd numbers from 1 to 9: 1+3+5+7+9 = 25
    let sum: i32 = 0;
    let i: i32 = 0;
    while (i < 10) {
        i = i + 1;
        if (i % 2 == 0) {
            continue;
        }
        sum = sum + i;
    }
    if (sum != 25) {
        return 1;
    }
    return 0;
}
