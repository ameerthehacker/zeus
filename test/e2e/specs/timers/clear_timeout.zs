function main(): i32 {
    let id: i32 = setTimeout(() => {
        console.log("should not fire");
    }, 10);
    clearTimeout(id);
    console.log("after clear");
    return 0;
}
