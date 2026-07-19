// for...of sums the elements of an array
function main(): i32 {
    let nums: i32[] = new i32[];
    nums.push(1);
    nums.push(2);
    nums.push(3);

    let sum: i32 = 0;
    for (const n of nums) {
        sum += n;
    }

    if (sum != 6) {
        return 1;
    }
    return 0;
}
