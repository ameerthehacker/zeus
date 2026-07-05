// Escaped var initialized from an indirect function call — type inferred from
// the callee's return type at IR gen time.
function makeCounter(fn: () => i32): () => i32 {
    let count = fn();
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let seed: () => i32 = (): i32 => { return 7; };
    let inc = makeCounter(seed);
    inc();
    inc();
    let val = inc();
    return val - 10;  // 7 + 3 = 10; exit 0 on success
}
