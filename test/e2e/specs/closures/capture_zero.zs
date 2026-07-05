function main(): i32 {
    let x: i32 = 0;
    function getX(): i32 { return x; }
    return getX();
}
