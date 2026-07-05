function makeAdder(n: i32): (x: i32) => i32 {
    function add(x: i32): i32 {
        return x + n;
    }
    return add;
}

function main(): i32 {
    let addThree: (x: i32) => i32 = makeAdder(3);
    return addThree(4);
}
