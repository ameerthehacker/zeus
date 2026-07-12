// A class may satisfy an interface property with a get/set accessor (not just a real field).
// Read dispatches to the getter and write to the setter through the object's own vtable.
interface Box {
  value: i32;
}

class Widget {
  private v: i32;
  constructor(v: i32) { this.v = v; }
  get value(): i32 { return this.v; }
  set value(x: i32) { this.v = x; }
}

function readIt(b: Box): i32 { return b.value; }        // getter dispatch
function writeIt(b: Box, x: i32): void { b.value = x; } // setter dispatch

function main(): i32 {
  let w: Widget = new Widget(10);
  let b: Box = w;               // accessor-backed conformer accepted as Box
  writeIt(b, 32);               // setter: v = 32
  return readIt(b) + b.value;   // getter twice → 32 + 32 = 64
}
