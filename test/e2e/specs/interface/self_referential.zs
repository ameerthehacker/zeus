interface Node {
  value: i32;
  next: Node;
}

// LinkedNode structurally conforms to the self-referential Node interface. Conformance
// checking must terminate (not recurse forever) on the `next: Node` cycle.
class LinkedNode {
  public value: i32;
  public next: Node;

  constructor(v: i32) {
    this.value = v;
  }
}

function useNode(n: Node): i32 {
  return 0;
}

function main(): i32 {
  let ln: LinkedNode = new LinkedNode(42);
  let n: Node = ln;

  return useNode(n) + ln.value;
}
