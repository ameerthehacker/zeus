// A default parameter is used when the argument is omitted, and overridden when provided.
function add(a: i32, b: i32 = 10): i32 {
    return a + b;
}

function main(): i32 {
    if (add(5) != 15) {
        return 1; // default used
    }
    if (add(5, 2) != 7) {
        return 2; // default overridden
    }
    return 0;
}
