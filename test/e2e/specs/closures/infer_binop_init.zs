// Escaped var initialized from a binary op expression — type must be inferred
// at IR gen time (not available via annotation or eager result).
function makeCounter(start: i32, step: i32): () => i32 {
    let count = start + step;
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let inc = makeCounter(10, 5);
    inc();
    inc();
    let val = inc();
    return val - 18;  // 15 + 3 = 18; exit 0 on success
}
