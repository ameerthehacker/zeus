// End-of-input path: with no stdin, the C readline() returns NULL and @std/readline maps it to "".
import { readLine } from "@std/readline";

function main(): i32 {
  let line: string = readLine("prompt> ");
  if (line.length != 0) {
    return 1;
  }
  return 0;
}
