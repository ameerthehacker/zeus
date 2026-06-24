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

## 2. Expression-index array writes are an invalid lvalue

**Bug:** Using an arithmetic expression as the index in an array write (`arr[expr] = value`) is rejected at compile time with `invalid lvalue in assignment`.

```zeus
// BROKEN — compile error: invalid lvalue in assignment
arr[j + 1] = arr[j];
```

**Workaround:** Use the `.set(index, value)` method, which accepts any expression as the index argument.

```zeus
// WORKS
arr.set(j + 1, arr[j]);
```

> **Note:** Expression-index **reads** (`arr[j + 1]`) compile and run correctly. Only writes are affected.

---

## 3. Free function names shadow class method names inside the class

**Bug:** If a free function and a class method share the same name, calls to the free function inside the class body are resolved to the method instead, producing an argument-count error.

```zeus
// BROKEN — inside BST, inOrderSum(this.root) resolves to the 0-arg method
function inOrderSum(node: BSTNode): i32 { ... }

class BST {
    public inOrderSum(): i32 {
        return inOrderSum(this.root);  // error: expected 0 arguments, found 1
    }
}
```

**Workaround:** Give free helper functions a distinct name (e.g. a `node` prefix) so they don't collide with method names.

```zeus
// WORKS
function nodeSum(node: BSTNode): i32 { ... }

class BST {
    public inOrderSum(): i32 {
        return nodeSum(this.root);  // unambiguous
    }
}
```

---

## 4. `from` is a reserved keyword

**Bug:** `from` cannot be used as a function parameter name. The parser rejects it with `expected identifier in function parameter, but found from`.

```zeus
// BROKEN — parse error
function hanoi(n: i32, from: i32, to: i32, aux: i32): i32 { ... }
```

**Workaround:** Rename the parameter.

```zeus
// WORKS
function hanoi(n: i32, src: i32, dst: i32, aux: i32): i32 { ... }
```

---

## 5. `null` cannot be explicitly assigned

**Observation:** Zeus does not expose `null` as a keyword you can write in source. Uninitialized class-typed fields are `null` at runtime, and comparing them with `== null` compiles and works correctly.

```zeus
// WORKS at runtime — null comparison is valid
if (this.head == null) { ... }
while (current.next != null) { ... }
```

However, there is no way to explicitly assign `null` to reset a reference. Use a sentinel value or boolean flag instead.
