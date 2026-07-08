function main(): i32 {
    let id: i32 = 0;
    let count: i32 = 0;
    id = setInterval(() => {
        count = count + 1;
        console.log("tick");
        if (count >= 3) {
            clearInterval(id);
        }
    }, 10);
    return 0;
}
