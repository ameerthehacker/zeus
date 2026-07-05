// Escaped var initialized from a ternary expression — type must be inferred
// at IR gen time.
function makeCounter(positive: boolean): () => i32 {
    let count = positive ? 10 : 0;
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let inc = makeCounter(true);
    inc();
    inc();
    let val = inc();
    return val - 13;  // 10 + 3 = 13; exit 0 on success
}
