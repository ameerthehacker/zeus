// Interpolating a value whose type does not implement Stringify (no toString) is still an error.
// (Numbers/bools/objects-with-toString now convert implicitly — see specs/stringify.)
function main(): i32 {
    let a: i32[] = [1, 2];
    let s: string = `value is ${a}`;
    return 0;
}
