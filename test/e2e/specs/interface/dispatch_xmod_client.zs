// The conformer (Circle) is defined HERE, in the client, and passed into a library function
// (areaOf) that dispatches through the interface. The dispatch site is compiled in the library
// module, which never sees Circle — yet dispatch must still land on Circle.area. `area` is at a
// non-trivial vtable slot (after foo/bar) so a wrong itable would call the wrong method.
import { Shape, areaOf } from "./client_lib";

class Circle {
  public r: i32;
  constructor(r: i32) { this.r = r; }
  public foo(): i32 { return 111; }   // vtable slot 0
  public bar(): i32 { return 222; }   // vtable slot 1
  public area(): i32 { return this.r * this.r; }   // vtable slot 2
}

function main(): i32 {
  return areaOf(new Circle(6));   // 6*6 = 36 (not 111/222 — those are wrong slots)
}
