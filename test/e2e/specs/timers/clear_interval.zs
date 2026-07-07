function main(): i32 {
    let id: i32 = 0;
    id = setInterval(() => {
        log("should not fire");
    }, 10);
    log("before clear");
    clearInterval(id);
    return 0;
}
