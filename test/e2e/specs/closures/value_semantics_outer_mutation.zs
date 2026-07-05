function main(): i32 {
    let x: i32 = 10;
    function getX(): i32 {
        return x;
    }
    x = 99;
    if (getX() == 99) {
        return 0;
    }
    return 1;
}
