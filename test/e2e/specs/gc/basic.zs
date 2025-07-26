class Example {
  public x: i32;

  constructor(x: i32) {
    this.x = x;
  }

  setX(x: i32): void {
    this.x = x;
  }
}

function dummy(e1: Example, e2: Example, b: i32): Example {
  e1.setX(12);
  e2.setX(13);

  return e2;
}

function dummy2(e1: Example, e2: Example, b: i32): i32 {
  let e3: Example = new Example(14);
  let e4: Example = new Example(15);
  e1.setX(12);
  return e2.x;
}

function dummy3():void {
}

function main(): i32 {
  let a: i32 = 10;
  let e1: Example = new Example(10);
  let e2: Example = new Example(11);

  dummy(e1, e2, 10);
  dummy2(e1, e2, 10);
  dummy3();
  return e2.x; 
}
