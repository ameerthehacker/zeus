function main(): i32 {
    let msg: string = "hello from closure";
    setTimeout(() => {
        log(msg);
    }, 0);
    return 0;
}
