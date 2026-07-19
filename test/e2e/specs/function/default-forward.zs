// A default parameter works even when the function is called before it is declared.
function main(): i32 {
    if (helper(5) != 15) {
        return 1;
    }
    return 0;
}

function helper(a: i32, b: i32 = 10): i32 {
    return a + b;
}
