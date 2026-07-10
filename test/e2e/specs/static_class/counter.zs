class Counter {
    public static count: i32;
    public static increment(): void {
        Counter.count = Counter.count + 1;
    }
    public static getCount(): i32 {
        return Counter.count;
    }
}
function main(): i32 {
    Counter.count = 0;
    Counter.increment();
    Counter.increment();
    return Counter.getCount();
}
