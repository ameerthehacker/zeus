// Multiple reads over one stdin stream, exercising add_history() between them, then EOF -> "".
import { readLine, addHistory } from "@std/readline";

function main(): i32 {
  let first: string = readLine("> ");
  if (!first.equals("first")) {
    return 1;
  }
  addHistory(first);

  let second: string = readLine("> ");
  if (!second.equals("second")) {
    return 2;
  }
  addHistory(second);

  // stream is now exhausted -> end-of-input -> ""
  let third: string = readLine("> ");
  if (third.length != 0) {
    return 3;
  }
  return 0;
}
