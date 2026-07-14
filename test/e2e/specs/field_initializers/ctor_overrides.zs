// Field initializers run before the constructor body, so the body can build on them. Here value
// starts at 5 (initializer), then the constructor adds 4 → 9. If the initializer had not run first,
// the result would be 0 + 4 = 4, so this exit code proves the ordering.
class Box {
    public value: i32 = 5;

    public constructor() {
        this.value = this.value + 4;
    }
}

function main(): i32 {
    let b: Box = new Box();
    return b.value;
}
