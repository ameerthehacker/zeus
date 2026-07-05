function main(): i32 {
    let x: i32 = 10;
    function getX(): i32 {
        return x;
    }
    x = 99;
    return getX();
}
