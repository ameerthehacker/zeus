// break exits a for...of loop early
function main(): i32 {
    let nums: i32[] = new i32[];
    nums.push(1);
    nums.push(2);
    nums.push(3);
    nums.push(4);

    let sum: i32 = 0;
    for (const n of nums) {
        if (n == 3) {
            break;
        }
        sum += n; // 1 + 2
    }

    if (sum != 3) {
        return 1;
    }
    return 0;
}
