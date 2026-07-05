function makeScale(factor: i32): (x: i32) => i32 {
    function scale(x: i32): i32 { return x * factor; }
    return scale;
}

function makeOffset(offset: i32): (x: i32) => i32 {
    function shift(x: i32): i32 { return x + offset; }
    return shift;
}

function main(): i32 {
    let double: (x: i32) => i32 = makeScale(2);
    let addTen: (x: i32) => i32 = makeOffset(10);
    let triple: (x: i32) => i32 = makeScale(3);
    if (double(7) == 14 && addTen(5) == 15 && triple(4) == 12 && double(addTen(3)) == 26) {
        return 0;
    }
    return 1;
}
