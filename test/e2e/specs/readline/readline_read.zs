// Success path: a line piped to stdin comes back with its trailing newline stripped.
import { readLine } from "@std/readline";

function main(): i32 {
  let line: string = readLine("name> ");
  if (!line.equals("hello from stdin")) {
    return 1;
  }
  return 0;
}
