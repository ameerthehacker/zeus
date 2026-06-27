function greet(name: string) {
  log("Hello, " + name);
}

class Counter {
  public value: i32;

  constructor(start: i32) {
    this.value = start;
  }

  public increment() {
    this.value = this.value + 1;
  }
}

function main(): i8 {
  greet("Zeus");
  let c: Counter = new Counter(0);
  c.increment();
  c.increment();

  return c.value - 2;
}
