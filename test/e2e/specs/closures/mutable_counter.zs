function makeCounter(): () => i32 {
    let count: i32 = 0;
    return (): i32 => {
        count += 1;
        return count;
    };
}

function main(): i32 {
    let incr: () => i32 = makeCounter();
    incr();
    incr();
    incr();
    if (incr() == 4) {
        return 0;
    }
    return 1;
}
