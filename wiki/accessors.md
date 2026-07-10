# Getters and Setters (Accessors)

Zeus supports TypeScript-style getter and setter accessors on classes. They look like property accesses to callers but invoke methods under the hood.

---

## Syntax

```zeus
class Circle {
    private _radius: float;

    get radius(): float {
        return this._radius;
    }

    set radius(value: float) {
        if (value < 0.0) { return; }
        this._radius = value;
    }
}

let c = new Circle();
c.radius = 5.0;       // calls setter
let r = c.radius;     // calls getter
c.radius += 1.0;      // GET + compute + SET
c.radius++;           // GET + compute + SET (postfix: returns old value)
```

### Rules enforced by the parser and type checker

| Rule | Where enforced |
|------|---------------|
| Getter must declare a non-void return type | Parser |
| Getter must have no parameters | Parser |
| Setter must have exactly one parameter | Parser |
| Accessor name must not clash with a data property | Type checker (`tcDeclClass`) |
| If both getter and setter exist, their types must match | Type checker (`tcDeclClass`) |
| Private accessor cannot be read/written outside the class | Type checker (`tcGetAccessor`/`tcSetAccessor`) |
| Reading a setter-only accessor → "write-only" error | Type checker |
| Writing a getter-only accessor → "read-only" error | Type checker |
| Compound assignment (`+=`) requires both getter and setter | IR gen (reads via getter, writes via setter) |

### Soft keywords

`get` and `set` are **not** reserved words. The parser recognises them as accessor prefixes only when the next two tokens are `identifier` followed by `(`. Existing methods named `get` or `set` continue to work.

---

## Access modifiers

Access modifiers (`public`/`private`) work the same way as on regular methods. The modifier applies to the accessor pair as a whole:

```zeus
class Foo {
    private _x: i32;
    private get x(): i32 { return this._x; }  // only accessible inside Foo
}
```

---

## Implementation overview

Accessors go through a dedicated HIR-to-LIR pipeline:

```
Parser (AST ClassMethod.Accessor)
  → IR gen (GET_ACCESSOR / SET_ACCESSOR HIR instructions)
    → Type checker (validates, resolves output types)
      → AccessorLoweringPass (replaces with CALL_METHOD)
        → Codegen (sees only CALL_METHOD — no new codegen cases)
```

### IR instructions

| Instruction | Input | Output | Notes |
|-------------|-------|--------|-------|
| `GET_ACCESSOR` | `GetAccessorInstrInput{Object, AccessorName}` | `*Var` (getter return value) | HIR only — lowered before codegen |
| `SET_ACCESSOR` | `SetAccessorInstrInput{Object, AccessorName, Value}` | `*Var` (the assigned value) | HIR only — lowered before codegen |

### Mangled function names

Accessor bodies are emitted as ordinary IR functions with compiler-reserved `#`-prefixed names. Because `#` is not a valid Zeus identifier character, users can never define a function with the same name.

| Accessor | IR function name |
|----------|-----------------|
| `get radius()` | `#get_radius` (uniquified if needed: `#get_radius_1`, …) |
| `set radius(v)` | `#set_radius` |

The mangled name is written back into `ClassAccessor.Getter.Name` / `.Setter.Name` during IR generation (`buildAccessors` in `ir.go`).

### Lowering

`AccessorLoweringPass` (registered before `FuncTypePropCallLoweringPass`) rewrites every `GET_ACCESSOR`/`SET_ACCESSOR`:

| HIR | LIR |
|-----|-----|
| `GET_ACCESSOR(obj, "radius")` | `CALL_METHOD(obj, "#get_radius", [])` |
| `SET_ACCESSOR(obj, "radius", v)` | `CALL_METHOD(obj, "#set_radius", [v])` |

**Primordial accessors** (`IsLowered = true`) skip the `CALL_METHOD` path and expand directly to field access — no Zig runtime function is needed:

| HIR | LIR |
|-----|-----|
| `GET_ACCESSOR(arr, "length")` | `OBJECT_PROPERTY_ACCESS(arr, "_length")` → `LOAD` |

### `this` bypass

Inside a getter or setter body, `this.propName` bypasses the accessor lookup entirely (`isThisExpression` check in `VisitObjectPropertyAccessExpr`). This prevents infinite recursion when the getter reads the backing field by name.

---

## Array `length` primordial accessor

The built-in array type exposes `length` as a read-only accessor backed by the private `_length` field:

```zeus
let arr = new i32[](4);
arr.push(1);
let n = arr.length;   // calls the length getter → reads _length directly
// arr.length = 5;    // type error: property 'length' is read-only
```

Internal compiler code (bounds checks, lowering passes) accesses `_length` directly via `ARRAY_PROPERTY_LENGTH` constant — it never goes through the getter.

---

## Adding a new primordial accessor

1. Add a private backing property (e.g. `_foo`) to the class definition in `primordials.go`.
2. Create a `*Function` with name `#get_foo` and the desired return type.
3. Create a `ClassAccessor{Name: "foo", Getter: fn, IsLowered: true}`.
4. Pass the accessor slice to `NewClass`.
5. The `AccessorLoweringPass` will automatically expand `GET_ACCESSOR(obj, "foo")` to `OBJECT_PROPERTY_ACCESS(obj, "_foo") + LOAD`.

No Zig runtime function or codegen case is needed.
