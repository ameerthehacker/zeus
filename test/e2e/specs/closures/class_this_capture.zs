class Box {
    public value: i32;
    constructor(v: i32) { this.value = v; }
    public getGetter(): () => i32 {
        function get(): i32 {
            return this.value;
        }
        return get;
    }
}

function main(): i32 {
    let b: Box = new Box(42);
    let getter: () => i32 = b.getGetter();
    return getter();
}
