function main(): i32 {
    let x: i32 = 7;
    function level1(): i32 {
        function level2(): i32 {
            function level3(): i32 {
                return x;
            }
            return level3();
        }
        return level2();
    }
    return level1();
}
