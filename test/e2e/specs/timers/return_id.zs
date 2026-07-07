function main(): i32 {
    let id: i32 = setTimeout(() => { }, 1);
    if (id > 0) {
        return 0;
    }
    return 1;
}
