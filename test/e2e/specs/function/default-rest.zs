// A rest parameter cannot have a default value.
function f(a: i32, ...rest: i32[] = []): i32 {
    return a;
}

function main(): i32 {
    return f(1);
}
