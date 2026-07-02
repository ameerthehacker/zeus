# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## Capturing outer variables in a function expression crashes the compiler

Referencing a variable from an enclosing scope inside an anonymous function, fat arrow, or named function expression (i.e. forming a closure) causes a SIGSEGV or SIGBUS during codegen. Closures are not yet implemented 

```zeus
// Bug: crashes — fat arrow closes over 'n' from the outer function
function makeAdder(n: i32): (x: i32) => i32 {
  return (x: i32): i32 => { return x + n; }
}
```

```zeus
// Bug: crashes — anonymous function closes over 'a'
function main(): i32 {
  let add = function(a: i32): (b: i32) => i32 {
    return (b: i32): i32 => { return a + b; }
  }
  return add(3)(7);
}
```

**Workaround:** Only reference the function's own parameters and locally declared variables inside a function expression. To carry external state, pass it as an explicit parameter or use a class instance.

```zeus
// OK: fat arrow only uses its own parameter
function makeDouble(): (x: i32) => i32 {
  return (x: i32): i32 => { return x * 2; }
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
