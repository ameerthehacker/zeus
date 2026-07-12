// Exercises the constants and transcendental/random methods with tolerance ranges (their results
// are not exactly representable). Returns 0 on success; distinct sentinels on failure.
function main(): i32 {
    if (Math.PI < 3.14159 || Math.PI > 3.14160) { return 1; }
    if (Math.E < 2.71828 || Math.E > 2.71829) { return 2; }

    let lnE: f64 = Math.log(Math.E);
    if (lnE < 0.9999 || lnE > 1.0001) { return 3; }

    if (Math.exp(0.0) != 1.0) { return 4; }
    if (Math.sin(0.0) != 0.0) { return 5; }
    if (Math.cos(0.0) != 1.0) { return 6; }

    let l2: f64 = Math.log2(8.0);
    if (l2 < 2.9999 || l2 > 3.0001) { return 7; }

    let l10: f64 = Math.log10(1000.0);
    if (l10 < 2.9999 || l10 > 3.0001) { return 8; }

    let cube: f64 = Math.cbrt(27.0);
    if (cube < 2.9999 || cube > 3.0001) { return 9; }

    let r: f64 = Math.random();
    if (r < 0.0 || r >= 1.0) { return 10; }

    return 0;
}
