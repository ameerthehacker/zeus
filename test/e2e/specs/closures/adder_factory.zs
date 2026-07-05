function makeAdder(n: i32): (x: i32) => i32 {
    function add(x: i32): i32 {
        return x + n;
    }
    return add;
}

function main(): i32 {
    let add5: (x: i32) => i32 = makeAdder(5);
    let add10: (x: i32) => i32 = makeAdder(10);
    if (add5(3) == 8 && add10(3) == 13 && add5(0) == 5 && add10(0) == 10) {
        return 0;
    }
    return 1;
}
