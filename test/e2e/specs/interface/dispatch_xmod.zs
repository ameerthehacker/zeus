import { Greeter, English } from "./greeter_lib";

// The interface (Greeter) and the concrete class (English) are both defined in another
// module. Dispatch reads the method from the object's own (imported) vtable at runtime.
function welcome(g: Greeter): i32 {
  return g.greet();
}

function main(): i32 {
  let e: English = new English(42);
  return welcome(e);
}
