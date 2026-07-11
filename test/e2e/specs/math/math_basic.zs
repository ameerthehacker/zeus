// Exercises the exact-valued Math methods. Returns 0 on success; each failure returns a
// distinct sentinel so a failing assertion is identifiable from the exit code.
function main(): i32 {
    if (Math.sqrt(16.0) != 4.0) { return 1; }
    if (Math.floor(3.7) != 3.0) { return 2; }
    if (Math.ceil(3.2) != 4.0) { return 3; }
    if (Math.round(3.5) != 4.0) { return 4; }
    if (Math.trunc(3.9) != 3.0) { return 5; }
    if (Math.abs(-5.0) != 5.0) { return 6; }
    if (Math.max(2.0, 9.0) != 9.0) { return 7; }
    if (Math.min(2.0, 9.0) != 2.0) { return 8; }
    if (Math.pow(2.0, 10.0) != 1024.0) { return 9; }
    if (Math.sign(-3.0) != -1.0) { return 10; }
    if (Math.hypot(3.0, 4.0) != 5.0) { return 11; }
    return 0;
}
