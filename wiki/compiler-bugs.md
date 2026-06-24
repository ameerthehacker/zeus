# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## 1. `&&` and `||` are not short-circuit evaluated

**Bug:** Both operands of `&&` and `||` are always evaluated, even when the result is determined by the left operand. This causes runtime crashes when the right side has a side effect (e.g. an out-of-bounds array read).

```zeus
// BROKEN — crashes with IndexOutOfBoundsException when j == -1
while (j >= 0 && arr[j] > key) { ... }
```

**Workaround:** Split the compound condition into a guard + inner `if`.

```zeus
// WORKS
while (j >= 0) {
    if (arr[j] > key) {
        // shift element
        j = j - 1;
    } else {
        j = -1;  // force exit
    }
}
```

---