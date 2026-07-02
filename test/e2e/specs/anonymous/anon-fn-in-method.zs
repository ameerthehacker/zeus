class Processor {
  public base: i32;
  constructor(b: i32) { this.base = b; }

  process(x: i32): i32 {
    let double = function(v: i32): i32 { return v * 2; };
    return double(x) + this.base;
  }
}

function main(): i32 {
  let p = new Processor(5);
  return p.process(5);
}
