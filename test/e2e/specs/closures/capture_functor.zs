function main(): i32 {
    let x: i32 = 3;
    function inner(): i32 {
        return x + 1;
    }
    function outer(): i32 {
        return inner() + 4;
    }
    return outer();
}
