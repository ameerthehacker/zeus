function main(): i32 {
    let x: i32 = 5;
    function modify(): i32 {
        x = 99;
        return x;
    }
    let inner_result: i32 = modify();
    if (inner_result == 99 && x == 5) {
        return 0;
    }
    return 1;
}
