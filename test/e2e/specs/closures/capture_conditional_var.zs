function main(): i32 {
    let result: i32 = 0;
    if (true) {
        let x: i32 = 42;
        function get(): i32 {
            return x;
        }
        result = get();
    }
    return result;
}
