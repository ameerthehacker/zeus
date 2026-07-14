const Counter = class {
  public count: i32;
  constructor(start: i32) {
    this.count = start;
  }
  getCount(): i32 {
    return this.count;
  }
};

// Ambient object global constructed with `new` and read across modules.
global counter = new Counter(41);

export function readCounter(): i32 {
  return counter.getCount();
}
