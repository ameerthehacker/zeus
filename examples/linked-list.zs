// Linked List Implementation
// Run with: make run file=linked-list
// Demonstrates: classes, object references, null handling

class Node {
  public value: i32;
  public next: Node;

  constructor(value: i32) {
    this.value = value;
    this.next = null;
  }
}

class LinkedList {
  public head: Node;
  public size: i32;

  constructor() {
    this.head = null;
    this.size = 0;
  }

  public append(value: i32): void {
    let newNode: Node = new Node(value);
    
    if (this.head == null) {
      this.head = newNode;
    } else {
      let current: Node = this.head;
      while (current.next != null) {
        current = current.next;
      }
      current.next = newNode;
    }
    
    this.size = this.size + 1;
  }

  public sum(): i32 {
    let total: i32 = 0;
    let current: Node = this.head;
    
    while (current != null) {
      total = total + current.value;
      current = current.next;
    }
    
    return total;
  }

  public length(): i32 {
    return this.size;
  }
}

function main(): i32 {
  let list: LinkedList = new LinkedList();
  
  // Add numbers 1 through 10
  let i: i32 = 1;
  while (i <= 10) {
    list.append(i);
    i = i + 1;
  }
  
  // Sum should be 55 (1+2+3+...+10)
  return list.sum();
}

