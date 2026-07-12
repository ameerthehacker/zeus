import { describe, it, assert } from "@std/test";

describe("arith", () => {
  it("adds", () => { assert(2 + 2 == 4, "addition is broken"); });
  it("muls", () => { assert(2 * 3 == 6, "multiplication is broken"); });
});
