# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## Unterminated string crashes LSP

```zeus
function main(): void {
  log("Hello World!)
}
```

If a string literal is never closed, the LSP crashes instead of reporting a clean error. The compiler itself handles it correctly (emits a lex error); only the LSP path is affected.

**Workaround:** always close string literals.

---

## ~~Integer literals are unsigned, so `0 - N` underflows~~ — FIXED

Previously bare integer literals were typed **unsigned** by magnitude, so `0 - 4` computed in `u8`
and wrapped to `252`. **Fixed:** integer literals now default to a **signed** int (floored at i32)
and adopt a narrower/target type only when the value fits (`let b: u8 = 200` ok, `let c: u8 = 300`
rejected). `0 - 4 == -4`. Float literals stay `f64`; there is no implicit float→int (`let x: i32 = 2.0`
is an error — use `2` or `2.0 as i32`).

**Remaining limitation:** constant-only *arithmetic* into a narrower-than-i32 type is not folded, so
`let x: u8 = 100 + 50` is a compile error even though 150 fits `u8` (the `100 + 50` is an i32
expression, not a single literal). Workaround: write the computed literal (`let x: u8 = 150`) or cast
(`(100 + 50) as u8`). A full fix would be compile-time constant folding.

---
