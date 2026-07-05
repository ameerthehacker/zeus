function makeNested(x: i32): (z: i32) => i32 {
    function middle(y: i32): (z: i32) => i32 {
        function inner(z: i32): i32 {
            return x + y + z;
        }
        return inner;
    }
    return middle(5);
}

function main(): i32 {
    let f: (z: i32) => i32 = makeNested(10);
    return f(0);
}
