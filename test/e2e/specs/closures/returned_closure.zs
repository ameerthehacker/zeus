function makeMultiplier(factor: i32): (x: i32) => i32 {
    function multiply(x: i32): i32 {
        return x * factor;
    }
    return multiply;
}

function main(): i32 {
    let times5: (x: i32) => i32 = makeMultiplier(5);
    return times5(6);
}
