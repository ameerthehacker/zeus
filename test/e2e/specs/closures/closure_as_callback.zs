function apply(f: (x: i32) => i32, x: i32): i32 {
    return f(x);
}

function main(): i32 {
    let multiplier: i32 = 4;
    function times(x: i32): i32 {
        return x * multiplier;
    }
    return apply(times, 3);
}
