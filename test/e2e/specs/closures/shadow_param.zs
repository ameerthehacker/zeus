function main(): i32 {
    let x: i32 = 5;
    function addX(x: i32): i32 {
        return x + x;
    }
    return addX(10);
}
