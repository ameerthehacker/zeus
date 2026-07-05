function main(): i32 {
    let x: i32 = 42;
    function get(): i32 {
        return x;
    }
    return get();
}
