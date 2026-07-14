import { readCounter } from "./counter.zs";

function main(): i32 {
  return readCounter() + 1;
}
