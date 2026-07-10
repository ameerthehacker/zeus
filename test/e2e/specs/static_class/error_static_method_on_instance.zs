class Foo { public static greet(): i32 { return 1; } }
function main(): i32 {
    let f = new Foo();
    return f.greet();
}
