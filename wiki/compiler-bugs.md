# Compiler Bugs & Gotchas

Known bugs and surprising limitations in the Zeus compiler, discovered during development and e2e testing. Each entry includes a workaround so you can write correct code today while the bug is open.

---

## ✅ Fixed on 2026-07-18

The following stress/integration findings were **fixed** in this pass (each has a regression spec in
`test/e2e/specs/stress_bugs/`; the entries below still describe the original bug + the now-superseded
workaround for history):

- **Object-element arrays across modules** — `_Object.*` primordial symbols are now `InternalLinkage`, so `Widget[]`/`string[]` in multiple linked modules no longer duplicate-symbol at link. (`codegen.go` `genClass`)
- **Cross-module static access** — a class's `__static_*` global is now external in its defining module and declared extern in users; reading/writing an imported class's static no longer ICEs.
- **Import-order false circular dependency** — module collection now uses a proper DFS (topological order + real cycle detection), independent of import order. (`compiler.go` `CollectDependencies`)
- **`Array.slice` bounds** — start/end are clamped to `[0, length]` and length is never negative (no over-read / segfault). (`lowering.go` `lowerArraySlice`)
- **Unsigned literal ≥ 2⁶³** — codegen parses unsigned constants with `ParseUint`. (`llvm_type.go`)
- **Accessor on an array-subscript receiver** — `arr[i].getter/setter` now resolves (GET_INDEX result is typed at IR-gen; the receiver of a member access evaluates as an rvalue).
- **Ternary branch unification** — `flag ? 1 : 2.0` now unifies to `f64` instead of fixing the type from the first branch.
- **Self / circular inheritance** — rejected at compile time with a "circular inheritance" error (was: compile + runtime segfault).
- **`throw` inside a `catch`** — the caught handler is popped before the catch body runs, so a re-throw propagates to the outer handler instead of looping forever. (`exception_runtime.zig`)
- **Integer divide/modulo by zero** — now throws a catchable `ArithmeticException` (was: silent UB). Float `/0` still yields `Infinity`.
- **f32 arithmetic with a float literal** — the literal adopts `f32` (`let b: f32 = a + 0.2` compiles).
- **Duplicate same-name import** — now a compile error instead of silent last-wins.
- **Interface → concrete downcast** — `interfaceValue as ConcreteClass` is allowed and runtime-checked (ClassCastException on mismatch).
- **Diagnostics** — arg-count errors point at the call site; calling a data field reports "not callable"; a base class used only via `extends` is no longer flagged "declared but not used"; scientific-notation float literals (`6.022e23`) parse.
- **Docs** — the function-type syntax example (`(T,T) => R`), the `log(x)`→`console.log(x)` examples, and the no-modifier field examples were corrected; encapsulation docs now note fields default to **private**.

**Kept as intended (docs updated, not compiler bugs):** fields default to private (write `public`);
arrays are invariant (`C[]` ≠ `Interface[]`); `**` always yields `f64`. **Known gaps (by design, this
pass):** no `number → string` conversion yet (so `console.log(n)` / `` `${n}` `` need a hand-written
`itoa`); `finally` is not supported yet.

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

# Stress-test findings (2026-07-14)

Bugs found by a stress campaign that pushed documented features past the happy paths the e2e suite
covers (overflow, degenerate inputs, feature combinations, and inputs that contradict the docs).
Every repro below was compiled and run against the `zeus` binary; observed exit codes / messages are
quoted. Severity tags: **crash** (hang/segfault), **miscompile** (wrong result silently),
**bad-error** (real error, confusing diagnostic), **gotcha** (surprising-but-arguable behavior).
Where a defect could be pinned deterministically it also has a regression spec under
`test/e2e/specs/stress_bugs/`.

> **Second pass (building real programs).** Beyond the isolated probes above, I wrote four
> ~150–250-line programs to test feature interaction at scale: an arithmetic expression evaluator,
> an async task queue, and a typed hash-map/interface-sort library **all compiled and ran
> correctly** — so arithmetic/number-formatting, the timer event loop + closures + capture-by-
> reference, string hashing/equality, and interface dispatch at scale are solid. The fourth program,
> a recursive-descent JSON parser, could not be compiled at all: it hit the codegen crash documented
> immediately below. (Zeus also has **no user-defined generics, no `switch`, and no `for…of`** — each
> a clean parse error, so those are missing features, not bugs.)

---

## ~~`object.field = array[index]` crashes the compiler~~ — **FIXED 2026-07-15 (PR #28)**

```zeus
class N { public v: i32; constructor() { this.v = 0; } }
function pick(a: i32[], k: i32): N {
  if (k == 1) { let n: N = new N(); n.v = a[0]; return n; }
  if (k == 2) { let n: N = new N(); n.v = a[1]; return n; }   // same-named `n`
  return new N();
}
```

Originally reported (wrongly) as "storing an array-element load into an object field crashes
codegen." The real root cause was **two locals with the same source name in sibling (non-overlapping)
scopes** — e.g. `let n` in two different `if` branches — receiving the *identical* IR name, because
the unique-name helper only de-duped against *currently in-scope* symbols (the first branch's `n` is
already out of scope when the second is declared). Codegen keys its value map by that name, so the
two `alloca`s collided; when a branch body is split across multiple basic blocks (an array-index
bounds-check forces the split, which is why array indexing appeared to be the trigger), a trailing
`load n` resolved to the *wrong* uninitialised alloca. Debug builds crashed in LLVM's register
coalescer (`LiveRange::join`, which runs no IR verifier); release builds silently miscompiled to a
null return. The "load into a local first" workaround only helped because it changed the block
structure — the underlying multi-branch program was still miscompiled. **Fixed** by giving every
local a module-unique IR name (`generateUniqueVarIRName` + `usedVarNames`) while symbol resolution
still uses the source name. Verified: the original JSON parser and all sibling-scope stress cases
(different types, 5-way collisions, closures capturing colliding names, loop captures) now compile
and run correctly.

---

## `throw` inside a `catch` block hangs forever — **crash**

```zeus
function main(): i32 {
  try {
    throw new Error("A", "first");
  } catch (e: Error) {
    throw new Error("B", "second");   // re-throw / wrap-and-rethrow
  }
}
```

Throwing a **new** exception from within a `catch` handler puts the program into an infinite loop
(observed: killed by both a CPU-time limit and a wall-clock alarm; never terminates). This happens
for every shape tried: re-throw left uncaught at `main`, re-throw caught by an outer `try` in the
same frame, and re-throw in a callee caught by the caller. The catch→wrap→rethrow pattern is one of
the most common uses of exceptions, so this is easy to hit. (No regression spec is added for this
one — a hanging program would hang the e2e harness, which has no per-test timeout.)

**Workaround:** don't `throw` from inside a `catch`. Convert the handler to return a sentinel /
error value and throw *after* the `try`/`catch` has fully unwound, or restructure to validate before
the `try`.

---

## `Array.slice(start, end)` does no bounds checking — **miscompile / crash**

```zeus
let a: i32[] = [1, 2, 3];
a.slice(1, 100);   // length 99  — reads 96 elements out of bounds
a.slice(0 - 1, 2); // length 3   — reads before the start of the array
a.slice(4, 1);     // SIGSEGV    — negative length -> new i32[-3]
```

`slice` computes `sliceLen = end - start` and allocates `new T[sliceLen]`, then copies that many
elements — with **no clamping of `start`/`end` to `[0, length]` and no check that `start <= end`**.
Consequences: `end > length` returns an oversized slice that over-reads past the array (garbage /
potential crash); a negative `start` under-reads before the array; `start > end` produces a negative
length and segfaults. The lowering code even documents the intended clamp it never performs —
`internal/ir/lowering.go` `lowerArraySlice`, whose header comment says `sliceLen = end - start
(clamped)` while the body is a bare `end - start`. Contrast: bracket indexing (`a[i]`) *is* bounds
checked and throws `IndexOutOfBoundsException`.

**Workaround:** clamp arguments yourself before calling `slice`:
`let s = start < 0 ? 0 : start; let e = end > a.length ? a.length as i32 : end;` and skip the call
when `s >= e`.

---

## Integer divide / modulo by zero is undefined behavior — **miscompile**

```zeus
function main(): i32 {
  let a: i32 = 10;
  let b: i32 = 0;
  let c: i32 = a / b;      // no exception; c is garbage
  console.log("survived"); // this prints
  return c;
}
```

Integer `/` and `%` by zero are **not** checked. The program does not throw, prints "survived", and
returns a garbage value. Critically it is **not catchable** — wrapping the division in `try { ... }
catch (e: Error) { ... }` does not catch anything (the `try` body runs to completion). This is
inconsistent with Zeus's other runtime safety checks (null-property access throws
`NullReferenceException`, out-of-bounds indexing throws `IndexOutOfBoundsException`), and with the
docs' own exception-handling example, which manually guards `b == 0` — implying the language does not
auto-throw. (Float division by zero correctly yields IEEE `Infinity`.) The same class of unchecked
native UB also applies to shifting by ≥ the type's bit width (`1 << 32` for `i32` returns garbage).

**Workaround:** guard the divisor before dividing — `if (b == 0) { throw new Error("MathError",
"divide by zero"); }` — exactly as the docs example does.

---

## Ternary result type is taken from the first branch only — **gotcha**

```zeus
let x: f64 = flag ? 1 : 2.0;   // ERROR: type 'f64' is not assignable to type 'i32'
let y: f64 = flag ? 2.0 : 1;   // OK — 1 widens to f64
```

A conditional expression fixes its result type from whichever branch is emitted first and then
requires the other branch to be assignable to it — there is no unification to the wider common type.
So `flag ? 1 : 2.0` fails (result inferred `i32`, then `2.0` won't narrow) while the logically
identical `flag ? 2.0 : 1` compiles. Two surprises for a normal user: the result is **branch-order
dependent**, and the error message names `i32` even though the target is `f64`. (Widening *within*
the integer family is fine in either order, e.g. `flag ? i32Val : u8Val`.)

**Workaround:** put the wider-typed branch first, or make both branches the same type
(`flag ? 1.0 : 2.0`).

---

## Self / circular inheritance is never diagnosed — **gotcha / crash**

```zeus
class A extends A {              // accepted silently
  public x: i32;
  constructor() { super(); this.x = 7; }
}
function main(): i32 { let a: A = new A(); return a.x; }   // SIGSEGV at runtime
```

A class that extends itself — or a cycle such as `class A extends B` + `class B extends A` — passes
compilation with no error. What the user hits next depends on the constructor: if they call
`super()` the program compiles and then **segfaults at runtime** (the constructor recurses into
itself forever); if they don't, they get the misleading `cannot use 'this' before calling super(...)`
error, which never mentions the real problem (the inheritance cycle). Inheritance cycles should be
rejected at declaration with a clear diagnostic.

**Workaround:** none needed for correct code — just be aware that a mistyped `extends` naming the
class itself (or forming a cycle) will not be caught by the compiler and can crash at runtime.

---

## `**` always produces `f64`, so integer exponentiation needs a cast — **gotcha**

```zeus
let n: i32 = 2 ** 10;         // ERROR: type 'f64' is not assignable to type 'i32'
let n: i32 = (2 ** 10) as i32; // OK -> 1024
```

The power operator always evaluates in `f64` (the docs note it "operates on f64"), so even a
pure-integer expression like `2 ** 10` cannot be assigned to an integer variable without an explicit
`as` cast. This trips up the very common case of computing an integer power (`1 << n` works, but
`2 ** n` does not).

**Workaround:** cast the result — `(base ** exp) as i32` — or use `1 << n` for powers of two.

---

## Template `${...}` / `+` with a number gives a misleading "binary operation" error — **bad-error**

```zeus
let n: i32 = 42;
let s: string = `value is ${n}`;   // ERROR: invalid operands of type 'string' and 'i32'
                                    //        for binary operation
```

There is no implicit number→string conversion and no built-in numeric formatter, so you cannot
interpolate or concatenate a number into a string. That restriction is defensible, but the
diagnostic is not: a template literal is lowered to `+` concatenation, so the user — who typed no
operator at all, only `${n}` — is told about an "invalid ... binary operation". A beginner's first
instinct ("print a number") dead-ends here with a confusing message and no suggested path.

**Workaround:** build the string from bytes via a `u8[]` (there is no decimal formatter yet), or keep
numbers out of `console.log` / template strings until number→string lands.

---

## Argument-count errors point at the callee's definition, not the call site — **bad-error**

```zeus
function add(a: i32, b: i32): i32 { return a + b; }
function main(): i32 {
  return add(1, 2, 3);   // error points at line 1 (add's definition), not here
}
```

`error: expected 2 arguments for function, but found 3` anchors to the function *declaration*
(`add` at `1:10`) rather than the offending *call*. For a prelude-backed callee it is worse: calling
`Math.min(1.0, 2.0, 3.0)` reports the location as the user's file at **line 27** — a line that does
not exist in the (4-line) source, because the span comes from `Math.min`'s definition inside the
injected prelude but is stamped with the user's filename. Either way the caret does not land on the
bad call.

**Workaround:** ignore the reported line for arg-count mismatches and look for the call with the
wrong number of arguments.

---

## `finally` produces a cryptic parse error — **bad-error**

```zeus
try { return 1; } catch (e: Error) { return 2; } finally { return 3; }
// error: expected ;, but found {
```

`finally` is a documented "coming soon" feature, but a user who tries it gets
`expected ;, but found {` pointing at the `{` after `finally`, with no hint that `finally` itself is
the unsupported construct.

**Workaround:** run cleanup code after the `try`/`catch` block instead of in a `finally`.

---

## Calling a non-function field reports "property not found" — **bad-error**

```zeus
class A { public x: i32; constructor() { this.x = 1; } }
let a: A = new A();
a.x();   // error: property x not found in class A
```

Invoking a field that exists but is not callable says the property is **not found**, which is
actively wrong — `x` is found, it is just an `i32` rather than a function. The message should say
the member is not callable.

**Workaround:** don't add `()` when reading a data field; the fix is at the call site.

---

## A base class used only via `extends` is flagged "declared but not used" — **bad-error**

```zeus
class Base { public x: i32; constructor() { this.x = 10; } }
class Derived extends Base { constructor() { super(); } }
// warning: class 'Base' is declared but not used
let d: Derived = new Derived();  // Base's ctor + field are used through here
```

The unused-symbol pass does not count `extends` (or `super()`, or inherited-field access) as a use,
so a base class that is only referenced as a superclass draws a spurious "declared but not used"
warning. The same false positive appears for an accessor that is only invoked polymorphically through
an interface.

**Workaround:** none — the warning is harmless noise; the code is correct.

---

# Full-steam findings (2026-07-15)

A follow-up pass after the sibling-scope crash was fixed (PR #28), pushing into areas the earlier
rounds skipped: numeric widths, module graphs, deep inheritance, GC/allocation pressure, and more
feature combinations. **Verified solid (no bugs):** 6-level inheritance + `super` chain +
polymorphism, 100k-object allocation churn, a 1,000,000-element array, 5,000 string concatenations,
diamond module imports, re-export chains, exceptions thrown 10 frames deep, object-valued ternaries
(`obj`/`obj` and `obj`/`null`), and interface methods that return the interface type. New defects
below.

---

## Unsigned literal ≥ 2⁶³ crashes the compiler — **crash**

```zeus
let a: u64 = 9223372036854775808;   // 2^63  ->  "internal compiler error"
```

Any `u64` literal from `2^63` up to `2^64-1` (the entire upper half of the type's range) crashes the
compiler with `zeus: error: an internal compiler error occured` (a recovered Go panic:
`cannot convert int constant string to int`). Literals up to `2^63-1` compile fine. Root cause:
codegen converts the constant with **signed** `strconv.ParseInt(value.Value, 0, 64)` at
`internal/codegen/llvm_type.go:122` (`toLLVMConstant`), which overflows for values that don't fit a
signed `int64`. `GetSignedIntSize` in `zeus_value/value.go` already anticipates this case and falls
back gracefully, but the codegen conversion path does not. So half of `u64` — and the top of `u32`
math that produces such constants — is unusable via literals.

**Workaround:** none clean for a bare `u64` literal ≥ 2⁶³; build the value by arithmetic
(`let a: u64 = 9223372036854775807; a = a + 1;`) instead of writing the literal directly.

---

## f32 arithmetic with a float literal is rejected — **gotcha**

```zeus
let a: f32 = 0.1;          // OK — a bare literal adopts f32
let b: f32 = a + 0.2;      // ERROR: type 'f64' is not assignable to type 'f32'
```

A bare float literal assigned to an `f32` works, but the moment an `f32` value is combined with a
float **literal** in arithmetic, the literal stays `f64`, the whole expression widens to `f64`, and
it can no longer be stored back into an `f32`. Because integer literals *do* adopt their target/
context type (`let x: u8 = 200`), users reasonably expect the same for floats — so this makes `f32`
almost unusable for real math, since every constant in an `f32` expression must be cast. (The bare-
literal case works, which matches the docs' `let precise: f32 = 3.14159;` example — the gap is
specifically literals inside `f32` arithmetic.)

**Workaround:** cast every float literal used with an `f32`: `let b: f32 = a + (0.2 as f32);`.

---

## An interface value cannot be downcast to a concrete implementer — **gotcha**

```zeus
interface Shape { area(): i32; }
class Circle { public r: i32; constructor(r: i32) { this.r = r; } public area(): i32 { return this.r * this.r; } }

let s: Shape = new Circle(5);
let c: Circle = s as Circle;   // ERROR: cannot cast 'Shape' to 'Circle'
```

`as` supports class→class downcasts (runtime-checked, throwing `ClassCastException` on mismatch), but
casting an **interface-typed value to a concrete class that implements it** is rejected at compile
time. This blocks a common pattern: you hold a `Shape[]`, you know an element is really a `Circle`,
and you want to reach a `Circle`-only member — there is no supported way to recover the concrete type
from an interface value.

**Workaround:** keep a reference to the concrete type alongside the interface, or add the needed
member to the interface so you never have to downcast.

---

## Duplicate same-name import is silently accepted (last wins) — **gotcha**

```zeus
import { val } from "./x1";   // val() => 1
import { val } from "./x2";   // val() => 2  — silently shadows the first
// val() now returns 2, with no error or warning
```

Importing the same identifier from two different modules compiles without any diagnostic; the second
import silently wins. An accidental duplicate (or a name clash between two modules) is easy to miss.

**Workaround:** none available (Zeus has no import aliasing yet); avoid importing colliding names,
and be aware the last `import` binding is the one that takes effect.

---

# Common-user friction (docs + first-program sweep, 2026-07-15)

To gauge what a *newcomer* actually hits, I (a) compiled **every one of the 257 ` ```zeus ` code
blocks in the docs** — 210 compiled, 0 crashed, and the failures triaged to the four issues below
plus expected noise (multi-file `import` examples, API-signature listings, and fragments that
reference an identifier defined in an earlier block) — and (b) wrote the canonical first programs
(print 1..N, FizzBuzz, "you are N years old"). The findings below are ordered by how early a user
trips over them.

---

## You cannot print a number or a boolean — **friction (blocks the first program)**

```zeus
function main(): i32 {
  for (let i: i32 = 1; i <= 5; i++) {
    console.log(i);          // error: argument 1 of type 'i32' does not match expected type 'string'
  }
  return 0;
}
```

`console.log` accepts **only** `string`, and there is no `number → string` conversion anywhere
(`toString`, `String(n)`, a numeric `console.log` overload — none exist), nor `string + number`
concatenation. The practical consequence: **FizzBuzz — the canonical first program — cannot be
written**, and neither can "print 1 to 10" or "you are 25 years old". This is almost certainly the
first thing every new user hits, and the errors (`... does not match expected type 'string'`,
`invalid operands ... for binary operation`) give no hint of the fix. The only way to print a number
today is to hand-write an integer-to-string routine that fills a `u8[]` with ASCII digits and
converts it to a `string`.

**Workaround (the `itoa` every program needs):**

```zeus
function itoa(n: i32): string {
  if (n == 0) { return "0"; }
  let v: i32 = n; let neg: boolean = false;
  if (v < 0) { neg = true; v = 0 - v; }
  let digits: u8[] = new u8[];
  while (v > 0) { digits.push(((v % 10) as u8) + 48); v = v / 10; }
  let out: u8[] = new u8[];
  if (neg) { out.push(45); }
  let i: i32 = (digits.length as i32) - 1;
  while (i >= 0) { out.push(digits[i]); i = i - 1; }
  let s: string = out; return s;
}
```

---

## Class fields default to private — **gotcha (docs examples don't compile)**

```zeus
class Person {
  name: string;                              // no access modifier
  constructor(name: string) { this.name = name; }
}
let p: Person = new Person("zeus");
p.name;   // error: property name is not accessible in class Person
```

A field declared without an access modifier is **private**, not public. Zeus presents as a
TypeScript-like language, where the default is *public*, so a user who writes `name: string` (as the
docs do in `classes.mdx`, `exception-handling.mdx`, and elsewhere) gets a confusing "not accessible"
error when reading the field from outside. Several documented examples fail to compile for exactly
this reason.

**Workaround:** always write `public` (or `private`/`protected`) explicitly on fields:
`public name: string;`.

---

## Scientific-notation float literals do not parse — **bug (documented but unimplemented)**

```zeus
let x: f64 = 6.022e23;   // error: expected ;, but found identifier
```

`types.mdx` lists scientific notation (`6.022e23`) among the supported numeric literal formats, but
the lexer stops at `6.022` and treats `e23` as a separate identifier. Any exponent-form float — very
common in scientific/engineering code — fails with a misleading "expected ;" error.

**Workaround:** write the expanded decimal, or compute it (`6.022 * ...`); there is no literal
exponent form.

---

## Documented function-type syntax is wrong — **docs bug**

```zeus
let operation: function(i32, i32): i32 = add;   // error: expected data type ... but found function
let operation: (i32, i32) => i32 = add;         // correct
```

`types.mdx` shows the function-type annotation as `function(i32, i32): i32`, which the parser
rejects; the working syntax is the arrow form `(i32, i32) => i32`. (Relatedly, `functions.mdx` uses a
bare `log(x)` in one example where the real call is `console.log(x)`.) These are documentation
errors rather than compiler bugs, but a user copying them hits a wall.

**Workaround:** use `(params) => ReturnType` for function types, and `console.log` for output.

---

# Integration findings (build-a-real-app pass, 2026-07-15)

To test whether the compiler "holds together" when *all* the features are used at once, I built a
multi-module RPG battle engine (interface + inheritance + `super` + overrides + static + get/set
accessors + encapsulation + functor + variadic + closures + object arrays + exceptions + `as`
downcast + bitwise/power/ternary + recursion). **Within a single module the entire combination
compiles and runs correctly** — the single-file version is a clean win. Everything that broke is in
the **module system** (three bugs), plus two intra-module accessor/covariance edges. Splitting the
program across files was, in fact, impossible without hitting all three module bugs.

---

## Object-element array types in more than one linked module fail to link — **crash (link)**

```zeus
// widgets.zs
export class Widget { public id: i32; constructor(id: i32) { this.id = id; } }
export function firstId(xs: Widget[]): i32 { return xs[0].id; }   // materializes Widget[]

// main.zs
import { Widget, firstId } from "./widgets";
function main(): i32 {
  let a: Widget[] = new Widget[];   // materializes Widget[] again, in this module
  a.push(new Widget(7));
  return firstId(a) - 7;
}
// ld: duplicate symbol '_Object.headerPtr' / '_Object.vTablePtr' / '_Object.objectTypeInfoPtr'
// zeus: error: failed to link object files
```

Materializing an **object-element array type** (`Widget[]`, `string[]`, any `T[]` whose element is an
object — primitive arrays like `i32[]` are unaffected) causes codegen to emit the primordial
`_Object.*` layout symbols **non-weak**. If two object files in the same link both materialize any
such array, the link fails with duplicate symbols. Since the entry module and virtually any
imported module both tend to use object arrays, **this makes realistic multi-module programs
unlinkable** — arguably the most impactful bug for real applications, and invisible to the e2e suite
because its module tests don't use arrays across files. (Marking those primordial symbols `linkonce`/
`weak` would fix it.)

**Workaround:** confine all object-array usage (including `string[]`) to a single module — usually
impractical; in practice, keep multi-file programs small or inline into one file until fixed.

---

## Cross-module static member access crashes the compiler — **crash (ICE)**

```zeus
// counter.zs
export class Counter { public static n: i32; public v: i32; constructor() { this.v = 0; } }

// main.zs
import { Counter } from "./counter";
Counter.n = 5;     // panic: symbol __static_Counter_n not found
```

Reading, writing, or calling a static method that touches a static field of an **imported** class
panics in codegen (`symbol __static_* not found`, `codegen.go:getSymbol`) → "internal compiler
error". The static's storage global is only emitted in the defining module; other modules reference
a symbol that was never generated there. Single-module statics work; the e2e suite only covers those.

**Workaround:** never touch a class's statics from another module. Wrap them in free functions
*defined in the same module as the class* (e.g. `export function bumpCounter(): void { Counter.n =
Counter.n + 1; }`) and call those from elsewhere.

---

## Import order triggers a false circular dependency — **bad-error**

```zeus
// leaf.zs  (imports nothing)   mid.zs  (imports leaf)
import { leaf } from "./leaf";   //  <-- importing the leaf FIRST...
import { mid }  from "./mid";    //  ...then a module that also imports it
// error: circular dependency detected      (there is no cycle)
```

If the entry imports a module **before** it imports an intermediate that also depends on that
module, the resolver falsely reports a cycle. Simply **swapping the two `import` lines** compiles
fine. This is order-dependent and easy to trigger accidentally — an import sorter (alphabetical
auto-format) can introduce or mask it, and the "circular dependency" message points at nothing real.

**Workaround:** order imports so a shared/leaf module is imported *after* the modules that depend on
it (or just reorder until the false cycle disappears).

---

## Accessor (get/set) on an array-subscript receiver fails — **bug**

```zeus
class C { private _v: i32; constructor(v: i32) { this._v = v; } get v(): i32 { return this._v; } }
let a: C[] = new C[];
a.push(new C(42));
a[0].v;         // error: property v not found in class C
// a[0].v = 9;  // even worse: error: invalid operands of type 'undefined' and 'null' ...
let c: C = a[0]; c.v;   // OK via a local
```

Reading or writing an **accessor** (`get`/`set`) directly on an array-subscript receiver
(`arr[i].accessor`) fails — the getter form reports "property not found", the setter form emits a
nonsensical type-checker error. Plain *fields* and *methods* on a subscript work; only accessors
break. (Arrays of objects with computed properties are common, so this bites easily.)

**Workaround:** hoist the element to a local first — `let c = arr[i]; c.accessor` — then read/write
the accessor on the local.

---

## Arrays are not covariant — **gotcha**

```zeus
interface D { d(): i32; }
class C { public v: i32; constructor(v: i32) { this.v = v; } public d(): i32 { return this.v; } }
function sum(items: D[]): i32 { /* ... */ }
let a: C[] = /* ... */;
sum(a);   // error: argument of type 'C[]' does not match expected type 'D[]'
```

A `C[]` is not accepted where a `D[]` is expected even though `C` conforms to `D`. This is *type-safe*
(array invariance, and sounder than TypeScript's covariant arrays), but it's surprising in a
structurally-typed language and blocks the common "pass a list of concrete objects as a list of the
interface" pattern.

**Workaround:** build the interface-typed array explicitly and push each element
(`let ds: D[] = new D[]; ds.push(c);` — single-element `C`→`D` assignment is allowed), or write the
function to take the concrete `C[]`.

---
