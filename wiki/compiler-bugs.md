# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## Unterminated string crashes LSP

```zeus
function main(): void {
  log("Hello World!)
}
```

---
