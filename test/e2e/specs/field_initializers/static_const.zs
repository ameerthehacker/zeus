// A static readonly field with an initializer becomes a constant-initialized backing global,
// accessible via the class without any instance — the same mechanism that lets Math.PI work.
class K {
    public static readonly N: i32 = 42;
}

function main(): i32 {
    return K.N;
}
