function hanoi(n: i32, src: i32, dst: i32, aux: i32): i32 {
    if (n == 0) {
        return 0;
    }
    return hanoi(n - 1, src, aux, dst) + 1 + hanoi(n - 1, aux, dst, src);
}

function main(): i32 {
    if (hanoi(0, 0, 2, 1) != 0) { return 1; }
    if (hanoi(1, 0, 2, 1) != 1) { return 2; }
    if (hanoi(2, 0, 2, 1) != 3) { return 3; }
    if (hanoi(3, 0, 2, 1) != 7) { return 4; }
    if (hanoi(4, 0, 2, 1) != 15) { return 5; }
    return 0;
}
