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

Nested functions are now scoped correctly — a function declared inside another function is only visible within its declaring function. Calling it from any other scope produces an `undefined identifier` error.

```zeus
function outer(): i32 {
  function helper(): i32 { return 5; }
  return helper();  // OK
}

function main(): i32 {
  return helper();  // Now correctly errors: undefined identifier 'helper'
}
```

---

## Function type annotations are not supported

The `function` keyword is not recognised as a data type by `consumeDataType` in the parser. Attempting to annotate a parameter or variable with a function type produces a parse error.

```zeus
// Bug: parse error — "expected data type in function parameter, but found function"
function apply(f: function(a: i32, b: i32): i32, x: i32, y: i32): i32 {
  return f(x, y);
}
```

**Workaround:** Omit the type annotation and rely on type inference. Pass named top-level functions directly as arguments; you cannot annotate a `let`/`const` variable with a function type today.

---

## Named function expression: the function name leaks into the outer scope

When a named function is used as an expression and assigned to a variable, **both** the variable name and the function name are declared in the same (outer) scope. This differs from JavaScript's named function expression semantics where the inner name is only visible inside the function body itself.

```zeus
function main(): i32 {
  let f = function foo(): i32 { return 42; };
  return foo();  // 'foo' is callable here — it was registered in the same scope as 'f'
}
```

**Workaround:** If you do not want the function name to be accessible by its declared name, use a unique name for the variable and the function expression.

---
