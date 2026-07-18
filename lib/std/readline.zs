// @std/readline — interactive line editing, bound to the C readline API via @extern("C").
//
//   import { readLine, addHistory } from "@std/readline";
//
// The binding targets libedit (linked with -ledit); macOS ships it in the SDK and it exposes the
// readline-compatible `readline`/`add_history` symbols, so no third-party install is needed. To use
// GNU readline instead, change the @link below to @link("readline", "<libdir>").
//
// readLine prints a prompt, reads one edited line, and returns it with the trailing newline
// stripped, or "" at end-of-input. When stdin is not a terminal it degrades to a plain line read.
// addHistory records a line so the Up-arrow recalls it at later prompts.

@link("edit");

// readline() returns a malloc'd buffer the caller must free; NULL signals end-of-input.
@extern("C", "readline")    function c_readline(prompt: cstr): cstr;
@extern("C", "add_history") function c_add_history(line: cstr): cint;

// readLine prompts, reads one line, and returns it (newline stripped). Returns "" at end-of-input.
export function readLine(prompt: string): string {
  let raw: cstr = c_readline(cStrFromString(prompt));
  if (cIsNull(raw as cptr) as i32 != 0) {
    return "";
  }
  let line: string = cStrToString(raw);
  cFree(raw as cptr); // free the buffer readline() handed us
  return line;
}

// addHistory records `line` in the in-memory history so it can be recalled at future prompts.
export function addHistory(line: string): void {
  c_add_history(cStrFromString(line));
}
