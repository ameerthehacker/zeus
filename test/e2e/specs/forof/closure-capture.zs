// A closure created inside a for-of loop shares the `let` element binding: a mutation the
// closure makes is observed in the same iteration (the element is promoted to a ref cell,
// matching how a C-style for loop variable is captured).
function main(): i32 {
    let arr: i32[] = new i32[];
    arr.push(10);

    let observed: i32 = 0;
    for (let elem of arr) {
        let bump = (): void => {
            elem = elem + 5;
        };
        bump();
        observed = elem; // 15
    }

    return observed - 15;
}
