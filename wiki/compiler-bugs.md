# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## Variable declared without a type annotation or initializer is a compile error

Every variable declaration must have either a type annotation or an inline initializer. A bare `let x;` without either is rejected at compile time.

```zeus
// Error: variable 'x' must have a type annotation or an initializer
function main(): i32 {
  let x;
  x = 5;
  return x;
}
```

Use a type annotation or an initializer:

```zeus
let x: i32;    // ✓ type annotation
let y = 42;    // ✓ initializer (type inferred)
let z: i32 = 0; // ✓ both
```

---

## Reassigning a closure variable to a different closure expression fails type checking

Every closure expression (fat arrow or anonymous function) compiles into a unique functor class with a generated name (`anonymousFunctor`, `anonymous1Functor`, …). Even when two closure expressions have exactly the same parameter and return types, the type checker treats their compiled types as distinct nominal types and rejects assignment between them.

```zeus
// Bug: type error — 'anonymous1Functor' is not assignable to 'anonymousFunctor'
function main(): i32 {
  let f: () => i32 = (): i32 => { return 1; };
  f = (): i32 => { return 2; };   // ← compile error
  return f();
}
```

**Workaround:** Assign different names to the closures and call through a conditional, or use named function declarations instead.

```zeus
// OK: two separate variables, pick at call site by rebinding
function main(): i32 {
  let f1: () => i32 = (): i32 => { return 1; };
  let f2: () => i32 = (): i32 => { return 2; };
  let f: () => i32 = f2;   // rebinding to an already-typed variable works
  if (f() != 2) { return 1; }
  return 0;
}
```

---

## Fat arrow with trailing semicolon fails inside `return`

When a fat arrow expression is the direct value of a `return` statement, a `;` after the closing `}` causes a parse error. The same pattern works when assigned to a variable.

```zeus
// Bug: parse error — "expected expression, but found ;"
function makeDouble(): (x: i32) => i32 {
  return (x: i32): i32 => { return x * 2; };
}
```

```zeus
// OK: omit the semicolon after the fat arrow body
function makeDouble(): (x: i32) => i32 {
  return (x: i32): i32 => { return x * 2; }
}

// OK: assign to a variable first, then return it
function makeDouble(): (x: i32) => i32 {
  let f = (x: i32): i32 => { return x * 2; };
  return f;
}
```

---

## Class fields with function types crash the compiler

Declaring a class property whose type is a function type (e.g. `public transform: (x: i32) => i32;`) causes codegen to panic with `cannot get default llvm value for type: zeus_value.FunctionType`. Calling `this.fnField(args)` inside a method produces "property not found" before even reaching codegen.

```zeus
// Bug: compiler panic — cannot get default llvm value for FunctionType
class Box {
  public transform: (x: i32) => i32;
  public val: i32;
  constructor(v: i32, f: (x: i32) => i32) {
    this.val = v;
    this.transform = f;
  }
  apply(): i32 {
    return this.transform(this.val);  // also: "property transform not found"
  }
}
```

**Workaround:** Pass the function as a method parameter each time, or store computed results as plain scalar fields instead of storing the function itself.

```zeus
// OK: pass the transform as an argument to the method
class Box {
  public val: i32;
  constructor(v: i32) { this.val = v; }
  apply(f: (x: i32) => i32): i32 { return f(this.val); }
}
function main(): i32 {
  let b = new Box(5);
  return b.apply((x: i32): i32 => { return x * 2; });  // 10
}
```

---

## `private` access modifiers are not enforced

Fields and methods declared `private` can be read and written from outside the class without a compile error. The keyword is parsed and stored but the type checker does not enforce visibility.

```zeus
class Wallet {
  private balance: i32;
  constructor(b: i32) { this.balance = b; }
}

function main(): i32 {
  let w = new Wallet(100);
  w.balance = 0;        // should error — but compiles and runs silently
  return w.balance;     // should error — but returns 0
}
```

**Workaround:** Enforce privacy by convention (prefix `_`) and rely on code review. There is no language-enforced alternative today.

---

## Nested function cannot shadow a top-level function with the same name

A `function` declaration inside a function body does not shadow a top-level function that has the same name. The top-level function is always resolved, so the local definition is unreachable.

The same applies to named function expressions: `let f = function helper(...) {...}` adds `helper` to the enclosing scope, but if a top-level `helper` already exists it is not shadowed.

```zeus
function helper(): i32 { return 1; }

function main(): i32 {
  function helper(): i32 { return 9; }  // intended local shadow — ignored
  return helper();  // Bug: calls the top-level helper, returns 1, not 9
}
```

**Workaround:** Use a distinct name for the nested function.

```zeus
function helper(): i32 { return 1; }

function main(): i32 {
  function localHelper(): i32 { return 9; }
  return localHelper();  // 9 ✓
}
```

---

## Unterminated string crashes LSP

```zeus
function main(): void {
  log("Hello World!)
}
```

---
