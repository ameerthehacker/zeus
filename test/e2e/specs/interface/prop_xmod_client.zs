// Cross-module PROPERTY dispatch: the conformer (Widget) is defined in the client and read
// through an interface by a library function. The library module has no Widget struct, so the
// property's byte offset is reconstructed from Widget's field layout. A leading `pad` field puts
// `value` at a non-zero offset, so a wrong offset would read the wrong field.
import { HasValue, readValue } from "./client_lib";

class Widget {
  public pad: i32;
  public value: i32;
  constructor(v: i32) { this.pad = 999; this.value = v; }
}

function main(): i32 {
  return readValue(new Widget(77));   // reads Widget.value → 77 (not 999)
}
