class Node {
  public value: i32;
  public next: Node;
  constructor(v: i32) { this.value = v; }
}

function sumChain(n: Node): i32 {
  return n.value + n.next.value;
}

function main(): i32 {
  try {
    let n: Node = new Node(10);
    let result: i32 = sumChain(n);
    return 1;
  } catch (e: Error) {
    return 0;
  }
}
