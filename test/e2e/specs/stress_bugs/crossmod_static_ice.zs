// BUG (wiki "Cross-module static member access crashes codegen"): reading/writing a static
// field of an imported class panics with "symbol __static_* not found". When fixed, compiles.
import { Counter } from "./xmod_static";
function main(): i32 {
  Counter.n = 5;
  return Counter.n - 5;
}
