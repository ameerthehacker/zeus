class Queue {
    public data: i32[];

    constructor() {
        this.data = new i32[];
    }

    public enqueue(value: i32): void {
        this.data.push(value);
    }

    public dequeue(): i32 {
        if (this.data.isEmpty()) {
            throw new Error("QueueUnderflow", "Queue is empty");
        }
        let value: i32 = this.data[0];
        this.data = this.data.slice(1, this.data.length);
        return value;
    }

    public front(): i32 {
        if (this.data.isEmpty()) {
            throw new Error("QueueUnderflow", "Queue is empty");
        }
        return this.data[0];
    }

    public size(): i32 {
        return this.data.length;
    }

    public isEmpty(): boolean {
        return this.data.isEmpty();
    }
}

function main(): i32 {
    let q: Queue = new Queue();

    if (!q.isEmpty()) { return 1; }

    q.enqueue(10);
    q.enqueue(20);
    q.enqueue(30);
    if (q.size() != 3) { return 2; }

    if (q.front() != 10) { return 3; }

    if (q.dequeue() != 10) { return 4; }
    if (q.dequeue() != 20) { return 5; }
    if (q.dequeue() != 30) { return 6; }

    if (!q.isEmpty()) { return 7; }

    let caught: boolean = false;
    try {
        q.dequeue();
    } catch (e: Error) {
        caught = true;
    }
    if (!caught) { return 8; }

    return 0;
}
