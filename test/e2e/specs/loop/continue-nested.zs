function main(): i32 {
    // continue in inner loop must not skip the outer loop body
    // inner: skip j==1; outer: always accumulates outerSum
    let outerSum: i32 = 0;
    let innerSum: i32 = 0;
    for (let i: i32 = 0; i < 3; i++) {
        outerSum = outerSum + 1;
        for (let j: i32 = 0; j < 3; j++) {
            if (j == 1) {
                continue; // skips j==1 in inner loop only
            }
            innerSum = innerSum + 1;
        }
    }
    // outerSum == 3 (outer ran 3 times)
    // innerSum == 6 (3 outer iters * 2 inner iters where j != 1)
    if (outerSum != 3) {
        return 1;
    }
    if (innerSum != 6) {
        return 2;
    }
    return 0;
}
