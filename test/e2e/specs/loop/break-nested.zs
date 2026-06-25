function main(): i32 {
    // break in inner loop must not exit the outer loop
    // outer counts 0..4, inner breaks immediately → outer runs 5 times
    let outerCount: i32 = 0;
    let innerCount: i32 = 0;
    for (let i: i32 = 0; i < 5; i++) {
        outerCount = outerCount + 1;
        for (let j: i32 = 0; j < 10; j++) {
            innerCount = innerCount + 1;
            break; // exits inner loop after first iteration
        }
    }
    if (outerCount != 5) {
        return 1;
    }
    if (innerCount != 5) {
        return 2;
    }
    return 0;
}
