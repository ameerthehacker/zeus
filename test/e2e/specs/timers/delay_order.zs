function main(): i32 {
    setTimeout(() => {
        log("100ms");
    }, 100);
    setTimeout(() => {
        log("10ms");
    }, 10);
    return 0;
}
