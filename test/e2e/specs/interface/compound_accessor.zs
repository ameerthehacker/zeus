// A compound assignment through an accessor-backed interface property must invoke the getter
// EXACTLY ONCE (read the old value); the trailing read that produces the assignment's result is
// forwarded from the just-stored value, not a second getter call. Widget's getter counts its
// invocations, so a double-getter would be observable.
interface Box {
  value: i32;
}

class Widget {
  private _v: i32;
  private _reads: i32;
  constructor(v: i32) {
    this._v = v;
    this._reads = 0;
  }
  get value(): i32 {
    this._reads = this._reads + 1;
    return this._v;
  }
  set value(x: i32) {
    this._v = x;
  }
  public reads(): i32 { return this._reads; }
  public raw(): i32 { return this._v; }
}

function bump(b: Box): void {
  b.value += 10; // getter once (read 5), setter once (write 15), result forwarded
}

function main(): i32 {
  let w: Widget = new Widget(5);
  bump(w);
  // forwarding: reads() == 1, raw() == 15  ->  1 * 100 + 15 = 115
  // (a double-getter would give reads() == 2 -> 215)
  return w.reads() * 100 + w.raw();
}
