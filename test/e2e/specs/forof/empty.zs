// for...of over an empty array runs zero iterations
function main(): i32 {
    let nums: i32[] = new i32[];

    let count: i32 = 0;
    for (const n of nums) {
        count += 1;
    }

    if (count != 0) {
        return 1;
    }
    return 0;
}
