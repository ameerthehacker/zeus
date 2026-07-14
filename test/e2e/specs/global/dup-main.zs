import { useDup } from "./dup-lib.zs";

global dup: i32 = 2;

function main(): i32 {
  return useDup();
}
