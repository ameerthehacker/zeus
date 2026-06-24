class Stack {
    public data: i32[];

    constructor() {
        this.data = new i32[];
    }

    public push(value: i32): void {
        this.data.push(value);
    }

    public pop(): i32 {
        if (this.data.isEmpty()) {
            throw new Error("StackUnderflow", "Stack is empty");
        }
        let value: i32 = this.data[this.data.length - 1];
        this.data = this.data.slice(0, this.data.length - 1);
        return value;
    }

    public peek(): i32 {
        if (this.data.isEmpty()) {
            throw new Error("StackUnderflow", "Stack is empty");
        }
        return this.data[this.data.length - 1];
    }

    public size(): i32 {
        return this.data.length;
    }

    public isEmpty(): boolean {
        return this.data.isEmpty();
    }
}

function main(): i32 {
    let s: Stack = new Stack();

    if (!s.isEmpty()) { return 1; }
    if (s.size() != 0) { return 2; }

    s.push(10);
    s.push(20);
    s.push(30);
    if (s.size() != 3) { return 3; }

    if (s.peek() != 30) { return 4; }
    if (s.size() != 3) { return 5; }

    if (s.pop() != 30) { return 6; }
    if (s.pop() != 20) { return 7; }
    if (s.pop() != 10) { return 8; }

    if (!s.isEmpty()) { return 9; }

    let caught: boolean = false;
    try {
        s.pop();
    } catch (e: Error) {
        caught = true;
    }
    if (!caught) { return 10; }

    return 0;
}
