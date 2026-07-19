// for...of with a `let` binding allows mutating the element copy in the body
function main(): i32 {
    let nums: i32[] = new i32[];
    nums.push(1);
    nums.push(2);
    nums.push(3);

    let sum: i32 = 0;
    for (let n of nums) {
        n = n * 2; // mutating the loop-local copy is allowed with `let`
        sum += n;  // 2 + 4 + 6
    }

    if (sum != 12) {
        return 1;
    }
    return 0;
}
