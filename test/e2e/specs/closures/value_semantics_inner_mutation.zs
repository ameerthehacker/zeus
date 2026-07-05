function main(): i32 {
    let x: i32 = 5;
    function modify(): i32 {
        x = 99;
        return x;
    }
    let result: i32 = modify();
    if (result == 99 && x == 99) {
        return 0;
    }
    return 1;
}
