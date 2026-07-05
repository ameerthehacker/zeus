function main(): i32 {
    let base: i32 = 5;
    let add: (x: i32) => i32 = (x: i32): i32 => { return x + base; };
    return add(4);
}
