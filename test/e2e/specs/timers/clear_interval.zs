function main(): i32 {
    let id: i32 = 0;
    id = setInterval(() => {
        console.log("should not fire");
    }, 10);
    console.log("before clear");
    clearInterval(id);
    return 0;
}
