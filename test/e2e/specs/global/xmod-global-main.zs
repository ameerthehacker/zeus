import { readIt } from "./reader.zs";

global appVersion: i32 = 42;

function main(): i32 {
  return readIt();
}
