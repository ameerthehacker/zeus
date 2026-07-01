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

## ~~Nested functions are hoisted to global scope~~ *(fixed)*

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

## ~~Two nested functions with the same name silently overwrite each other~~ *(fixed)*

Two different outer functions can now each declare a nested function with the same source name. The IR builder assigns a unique IR-level name (e.g. `helper` and `helper1`) while both are still callable within their respective scopes by the original name `helper`. No LLVM name collision occurs.

```zeus
// Now works correctly — each outer function has its own independent "helper"
function foo(): string {
  function helper(): string { return "Hello "; }
  return helper();
}
function bar(): string {
  function helper(): string { return "World!"; }
  return helper();
}
function main(): i32 {
  log(foo() + bar())  // prints "Hello World!"
  return 0;
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

## Anonymous functions and classes must have a name

The parser always requires an identifier after the `function` and `class` keywords, even when used as expressions. Truly unnamed forms produce a parse error.

```zeus
// Bug: parse error — "expected identifier for function name, but found ("
let f = function(): i32 { return 1; };

// Bug: parse error — "expected identifier class name, but found {"
let obj = new class { public x: i32; constructor(v: i32) { this.x = v; } }(10);
```

**Workaround:** Always supply a name — the name is required even in expression position:

```zeus
// OK
let f = function myFn(): i32 { return 1; };

// OK
let obj = new class MyClass { public x: i32; constructor(v: i32) { this.x = v; } }(10);
```

Note: you must also terminate the `let`/`const` declaration with a `;` after the closing `}` of the function or class body, otherwise the variable declaration parser keeps consuming tokens.

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

## Debug `fmt.Println` fires when a constructor has a non-void return type

`internal/ir/ir.go` (inside `VisitClassDeclExpr`) contains a leftover `fmt.Println(method.ReturnType.Span)` debug call that executes whenever a constructor is declared with a non-void return type. The proper error is still raised, but a raw span struct is also printed to stdout.

```zeus
class Foo {
  public x: i32;
  // Bug: triggers error AND prints "{Start:{Line:3,Column:22}, End:{Line:3,Column:24}}" to stdout
  constructor(v: i32): i32 { this.x = v; }
}
```

**Workaround:** Remove the `fmt.Println` call at `ir.go` inside the constructor return-type check block. Until fixed, ignore the extra stdout line.

---

## Nested functions see outer locals, but not as true closures

A function declared inside another function can reference variables from the enclosing scope during IR emission (the symbol table is still active). However, the nested function is compiled as a **module-level LLVM function**, not a closure. It accesses the outer function's stack frame directly.

This is safe only when the inner function is called within the same invocation of the outer function. Storing the inner function in a variable and calling it after the outer function has returned would produce undefined behaviour (use-after-free of a stack frame).

```zeus
function main(): i32 {
  let x: i32 = 7;
  function inner(): i32 { return x; }  // 'x' is resolved at IR-emit time
  return inner();                        // safe: called within main's lifetime
}
```

**Workaround:** Only call nested functions within the lifetime of the function that declares them. Do not return or store nested function references that capture outer locals.

---

## Semicolons are required after function/class body when used in a `let`/`const` declaration

When a function or class expression is used as the initializer of a `let` or `const` statement, the declaration loop in `parseVarDeclStmt` terminates only when it sees a `;`. The closing `}` of the function body is not treated as an implicit terminator. Omitting the semicolon causes parse errors on the next statement.

```zeus
// Bug: missing semicolon after the function body — parser keeps consuming tokens
let f = function foo(): i32 { return 1; }
return f();  // "expected identifier in variable declaration, but found return"

// OK: explicit semicolon terminates the declaration
let f = function foo(): i32 { return 1; };
return f();
```

**Workaround:** Always write `;` after the closing `}` of a function or class body when used as a `let`/`const` initializer.
