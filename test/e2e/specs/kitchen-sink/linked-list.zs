class Node {
    public value: i32;
    public next: Node;

    constructor(value: i32) {
        this.value = value;
    }
}

class LinkedList {
    public head: Node;
    public size: i32;

    constructor() {
        this.size = 0;
    }

    public append(value: i32): void {
        let node: Node = new Node(value);
        if (this.head == null) {
            this.head = node;
        } else {
            let current: Node = this.head;
            while (current.next != null) {
                current = current.next;
            }
            current.next = node;
        }
        this.size = this.size + 1;
    }

    public prepend(value: i32): void {
        let node: Node = new Node(value);
        node.next = this.head;
        this.head = node;
        this.size = this.size + 1;
    }

    public remove(value: i32): boolean {
        if (this.head == null) {
            return false;
        }
        if (this.head.value == value) {
            this.head = this.head.next;
            this.size = this.size - 1;
            return true;
        }
        let current: Node = this.head;
        while (current.next != null) {
            if (current.next.value == value) {
                current.next = current.next.next;
                this.size = this.size - 1;
                return true;
            }
            current = current.next;
        }
        return false;
    }

    public contains(value: i32): boolean {
        let current: Node = this.head;
        while (current != null) {
            if (current.value == value) {
                return true;
            }
            current = current.next;
        }
        return false;
    }

    public length(): i32 {
        return this.size;
    }

    public sumAll(): i32 {
        let sum: i32 = 0;
        let current: Node = this.head;
        while (current != null) {
            sum += current.value;
            current = current.next;
        }
        return sum;
    }
}

function main(): i32 {
    let list: LinkedList = new LinkedList();

    if (list.length() != 0) { return 1; }

    list.append(10);
    list.append(20);
    list.append(30);
    list.append(40);
    list.append(50);
    if (list.length() != 5) { return 2; }

    if (!list.contains(30)) { return 3; }
    if (list.contains(99)) { return 4; }

    if (list.sumAll() != 150) { return 5; }

    list.prepend(5);
    if (list.head.value != 5) { return 6; }

    if (!list.remove(30)) { return 7; }
    if (list.length() != 5) { return 8; }
    if (list.contains(30)) { return 9; }

    if (list.remove(99)) { return 10; }

    return 0;
}
