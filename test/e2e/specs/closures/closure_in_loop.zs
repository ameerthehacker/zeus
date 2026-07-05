function main(): i32 {
    let i: i32 = 1;
    let f1: () => i32 = function(): i32 { return i; };
    i = 2;
    let f2: () => i32 = function(): i32 { return i; };
    i = 3;
    let f3: () => i32 = function(): i32 { return i; };
    if (f1() == 3 && f2() == 3 && f3() == 3) {
        return 0;
    }
    return 1;
}
