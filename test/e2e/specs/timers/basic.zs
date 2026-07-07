function main(): i32 {
    setTimeout(() => {
        log("timer fired");
    }, 0);
    log("main done");
    return 0;
}
