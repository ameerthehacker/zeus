class BSTNode {
    public value: i32;
    public left: BSTNode;
    public right: BSTNode;

    constructor(value: i32) {
        this.value = value;
    }
}

function nodeInsert(node: BSTNode, value: i32): BSTNode {
    if (node == null) {
        return new BSTNode(value);
    }
    if (value < node.value) {
        node.left = nodeInsert(node.left, value);
    } else if (value > node.value) {
        node.right = nodeInsert(node.right, value);
    }
    return node;
}

function nodeContains(node: BSTNode, value: i32): boolean {
    if (node == null) {
        return false;
    }
    if (value == node.value) {
        return true;
    }
    if (value < node.value) {
        return nodeContains(node.left, value);
    }
    return nodeContains(node.right, value);
}

function nodeSum(node: BSTNode): i32 {
    if (node == null) {
        return 0;
    }
    return nodeSum(node.left) + node.value + nodeSum(node.right);
}

function nodeMax(a: i32, b: i32): i32 {
    if (a > b) {
        return a;
    }
    return b;
}

function nodeHeight(node: BSTNode): i32 {
    if (node == null) {
        return 0;
    }
    return 1 + nodeMax(nodeHeight(node.left), nodeHeight(node.right));
}

class BST {
    public root: BSTNode;

    constructor() {
    }

    public insert(value: i32): void {
        this.root = nodeInsert(this.root, value);
    }

    public contains(value: i32): boolean {
        return nodeContains(this.root, value);
    }

    public inOrderSum(): i32 {
        return nodeSum(this.root);
    }

    public height(): i32 {
        return nodeHeight(this.root);
    }
}

function main(): i32 {
    let tree: BST = new BST();

    if (tree.contains(5)) { return 1; }

    tree.insert(5);
    tree.insert(3);
    tree.insert(7);
    tree.insert(1);
    tree.insert(4);
    tree.insert(6);
    tree.insert(8);

    if (!tree.contains(5)) { return 2; }
    if (!tree.contains(1)) { return 3; }
    if (!tree.contains(8)) { return 4; }
    if (tree.contains(9)) { return 5; }

    if (tree.inOrderSum() != 34) { return 6; }

    tree.insert(5);
    if (tree.inOrderSum() != 34) { return 7; }

    if (tree.height() != 3) { return 8; }

    return 0;
}
