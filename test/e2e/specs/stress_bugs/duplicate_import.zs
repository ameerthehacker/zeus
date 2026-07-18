// REGRESSION (fixed): importing the same name from two modules is now a compile error
// instead of silently letting the last import win.
import { val } from "./xdup_a";
import { val } from "./xdup_b";
function main(): i32 { return val(); }
