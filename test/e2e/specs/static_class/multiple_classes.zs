class A { public static x: i32; }
class B { public static x: i32; }
function main(): i32 {
    A.x = 10;
    B.x = 20;
    return A.x + B.x;
}
