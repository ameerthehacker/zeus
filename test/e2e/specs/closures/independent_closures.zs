function main(): i32 {
    let x: i32 = 10;
    function getDouble(): i32 { return x * 2; }
    function getTriple(): i32 { return x * 3; }
    if (getDouble() == 20 && getTriple() == 30) {
        return 0;
    }
    return 1;
}
