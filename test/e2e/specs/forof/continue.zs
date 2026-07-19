// continue skips iterations of a for...of loop
function main(): i32 {
    let nums: i32[] = new i32[];
    nums.push(1);
    nums.push(2);
    nums.push(3);
    nums.push(4);

    let sum: i32 = 0;
    for (const n of nums) {
        if (n % 2 == 0) {
            continue;
        }
        sum += n; // 1 + 3
    }

    if (sum != 4) {
        return 1;
    }
    return 0;
}
