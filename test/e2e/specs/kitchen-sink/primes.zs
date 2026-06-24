function sieve(limit: i32): i32[] {
    let composite: i32[] = new i32[limit + 1];

    let result: i32[] = new i32[];

    for (let p: i32 = 2; p <= limit; p++) {
        if (composite[p] == 0) {
            result.push(p);
            let j: i32 = p * p;
            while (j <= limit) {
                composite[j] = 1;
                j = j + p;
            }
        }
    }

    return result;
}

function main(): i32 {
    let primes: i32[] = sieve(30);

    if (primes.length != 10) { return 1; }
    if (primes[0] != 2) { return 2; }
    if (primes[1] != 3) { return 3; }
    if (primes[4] != 11) { return 4; }
    if (primes[9] != 29) { return 5; }
    if (primes.includes(4)) { return 6; }
    if (primes.includes(25)) { return 7; }

    let small: i32[] = sieve(2);
    if (small.length != 1) { return 8; }

    return 0;
}
