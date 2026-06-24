class Counter {
  public count: i32;

  constructor() {
    this.count = 0;
  }

  public postfixIncrement(): i32 {
    return this.count++;
  }

  public prefixIncrement(): i32 {
    return ++this.count;
  }

  public postfixDecrement(): i32 {
    return this.count--;
  }

  public prefixDecrement(): i32 {
    return --this.count;
  }
}

function main(): i32 {
  let c: Counter = new Counter();

  // count=0: postfix returns old value (0), count becomes 1
  let old: i32 = c.postfixIncrement();

  // count=1: prefix returns new value (2), count becomes 2
  let new_: i32 = c.prefixIncrement();

  // count=2: postfix returns old value (2), count becomes 1
  c.postfixDecrement();

  // count=1: prefix returns new value (0), count becomes 0
  c.prefixDecrement();

  // exit code: old(0) + new_(2) + count(0) = 2
  return old + new_ + c.count;
}
