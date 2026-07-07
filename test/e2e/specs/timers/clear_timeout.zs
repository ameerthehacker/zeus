function main(): i32 {
    let id: i32 = setTimeout(() => {
        log("should not fire");
    }, 10);
    clearTimeout(id);
    log("after clear");
    return 0;
}
