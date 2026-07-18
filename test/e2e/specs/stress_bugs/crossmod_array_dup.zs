// BUG (wiki "Object-element array types in more than one linked module fail to link"):
// materializing an object-element array (Widget[]) in an imported module emits the _Object.*
// primordial symbols non-weak, so entry + module both define them -> link failure. When
// fixed, this links and returns 0.
import { Widget, firstId } from "./xmod_uses_array";
function main(): i32 {
  let a: Widget[] = new Widget[];
  a.push(new Widget(7));
  return firstId(a) - 7;
}
