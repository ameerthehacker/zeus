import { describe, it, assert } from "@std/test";

describe("truth", () => {
  it("ok", () => { assert(true, "should be true"); });
  it("bad", () => { assert(false, "boom"); });
});
