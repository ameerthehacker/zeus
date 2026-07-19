// A required parameter cannot follow an optional (defaulted) one.
function bad(a: i32 = 1, b: i32): i32 {
    return a + b;
}

function main(): i32 {
    return bad(1, 2);
}
