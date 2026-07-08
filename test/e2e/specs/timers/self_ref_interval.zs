function main(): i32 {
    let count: i32 = 0;
    let timer = setInterval(() => {
        if (count >= 3) {
            clearInterval(timer);
        } else {
            count = count + 1;
            console.log("tick");
        }
    }, 10);
    return 0;
}
