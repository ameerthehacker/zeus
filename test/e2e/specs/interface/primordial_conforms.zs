// Non-array primordial classes (string, Error, ...) are ordinary prelude classes with real
// vtables, so they participate in interface dispatch like any user class. Here `string`
// structurally satisfies both a method interface (concat) and a property interface (length),
// and is dispatched dynamically through them.
interface Concatable {
  concat(other: string): string;
}

interface HasLength {
  length: i32;
}

function joined(c: Concatable): string {
  return c.concat("!");
}

function lengthOf(h: HasLength): i32 {
  return h.length;
}

function main(): i32 {
  let s: string = "hi";
  let j: string = joined(s);       // method dispatch through interface → "hi!"
  return lengthOf(s) + lengthOf(j); // property dispatch through interface → 2 + 3 = 5
}
