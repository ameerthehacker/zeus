// nested for...of loops over two arrays
function main(): i32 {
    let a: i32[] = new i32[];
    a.push(1);
    a.push(2);
    let b: i32[] = new i32[];
    b.push(10);
    b.push(20);

    let sum: i32 = 0;
    for (const x of a) {
        for (const y of b) {
            sum += x * y; // (1*10 + 1*20) + (2*10 + 2*20) = 90
        }
    }

    if (sum != 90) {
        return 1;
    }
    return 0;
}
