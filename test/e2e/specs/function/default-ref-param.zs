// A default value cannot reference another parameter (defaults are substituted at the call site).
function f(a: i32, b: i32 = a): i32 {
    return a + b;
}

function main(): i32 {
    return f(1);
}
