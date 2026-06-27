class Point {
  public x: i32;
  public y: i32;

  constructor(x: i32, y: i32) {
    this.x = x;
    this.y = y;
  }

  public distSq(): i32 {
    return this.x * this.x + this.y * this.y;
  }
}

function add(a: i32, b: i32): i32 {
  return a + b;
}

function isEven(n: i32): boolean {
  return n % 2 == 0;
}

function makePoint(x: i32, y: i32): Point {
  return new Point(x, y);
}

function buildArray(n: i32): i32[] {
  let arr = new i32[];
  for (let i: i32 = 0; i < n; i++) {
    arr.push(i * 10);
  }
  return arr;
}

function greet(name: string): string {
  return "hi " + name;
}

function main(): i8 {
  // infer from integer literal
  let a = 10;
  let b = 3;

  // infer from boolean literal
  let flag = true;

  // infer from float literal
  let ratio = 2.5;

  // infer from string literal
  let lang = "zeus";

  // explicit annotation where signedness/size matters
  let small: i8 = 127;
  let big: i32 = 1000000;

  // infer from function call returning i32
  let sum = add(a, b);

  // infer from function call returning boolean
  let even = isEven(sum);

  // infer from object creation
  let p = makePoint(3, 4);

  // infer from array creation
  let nums = new i32[];
  nums.push(100);
  nums.push(200);
  nums.push(300);

  // infer from array element access
  let second = nums.get(1);

  // infer from function returning array
  let seq = buildArray(5);
  let fourth = seq.get(3);

  // infer from method call on object
  let dsq = p.distSq();

  // infer from string function call
  let msg = greet(lang);

  // infer from array length property
  let len = nums.length;

  // verify all inferred types behave correctly
  if (sum != 13) {
    return 1;
  }
  if (even) {
    return 2;
  }
  if (dsq != 25) {
    return 3;
  }
  if (second != 200) {
    return 4;
  }
  if (fourth != 30) {
    return 5;
  }
  if (!flag) {
    return 6;
  }
  if (len != 3) {
    return 7;
  }
  if (small != 127) {
    return 8;
  }
  if (big != 1000000) {
    return 9;
  }
  if (ratio < 2.0) {
    return 10;
  }

  return 0;
}
