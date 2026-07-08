function main(): i32 {
    setTimeout(() => {
        console.log("100ms");
    }, 100);
    setTimeout(() => {
        console.log("10ms");
    }, 10);
    return 0;
}
