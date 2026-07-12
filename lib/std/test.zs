// @std/test — a minimal Jest-like test API.
//
//   import { describe, it, assert } from "@std/test";
//
//   describe("math", () => {
//     it("adds", () => { assert(2 + 2 == 4, "addition is broken"); });
//   });
//
// Each `it` runs to completion; a failing `assert` throws and is reported as a failure without
// stopping the other tests. The functions print \x1f-delimited "ZTEST" marker lines that the
// `zeus test` runner parses to render the report and compute the exit code. Run a *.test.zs file
// directly (not via `zeus test`) and you'll just see the raw markers interleaved with any output.
//
// Output goes through a locally-constructed Console: the global `console` instance only exists in
// the entry module, but the `Console` class is a primordial available in every module.

export function describe(name: string, fn: () => void): void {
  const out: Console = new Console();
  out.log("\x1fZTEST\x1fENTER\x1f" + name);
  fn();
  out.log("\x1fZTEST\x1fEXIT\x1f");
}

export function it(name: string, fn: () => void): void {
  const out: Console = new Console();
  try {
    fn();
    out.log("\x1fZTEST\x1fPASS\x1f" + name);
  } catch (e: Error) {
    out.log("\x1fZTEST\x1fFAIL\x1f" + name + "\x1f" + e.message);
  }
}

export function assert(cond: boolean, message: string): void {
  if (!cond) {
    throw new Error("AssertionError", message);
  }
}
