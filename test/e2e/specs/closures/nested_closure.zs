function main(): i32 {
    let x: i32 = 42;
    function middle(): i32 {
        function inner(): i32 {
            return x;
        }
        return inner();
    }
    return middle();
}
