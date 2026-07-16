// Pure scalar arithmetic must be untouched by autoboxing — no boxing, no method dispatch. The exit
// code is the sum 0+1+...+9 = 45, computed entirely on unboxed i32.
function main(): i32 {
    let sum: i32 = 0;
    let i: i32 = 0;
    while (i < 10) {
        sum += i;
        i += 1;
    }
    return sum;
}
