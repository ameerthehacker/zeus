# JS Macro System — Roadmap & Feature Catalog

Zeus embeds [QuickJS](https://bellard.org/quickjs/) to run `.macro.js` files at compile time.
Macros receive typed Zeus AST arguments and emit Zeus source that gets parsed, type-checked, and compiled normally — no runtime cost, full compiler enforcement.

## How It Works

```
.zs source
   │
   ▼
 Parser  ──── sees macro call ────► QuickJS runs .macro.js
   │                                        │
   │          emitted Zeus source ◄─────────┘
   │                │
   ▼                ▼
  IR gen  ◄──── re-parsed + spliced in
   │
   ▼
 Type check, codegen, LLVM
```

Import from a `.macro.js` file — the extension is the signal. Calls look identical to normal function calls. Nothing from the macro file survives into the binary.

```zeus
import { table, fromFile } from "./orm.macro.js"
```

### The `ctx` API every macro receives

```js
export function myMacro(ctx, ...args) {
  ctx.arg(n).asString()    // extract string literal from nth arg
  ctx.arg(n).asFields()    // extract [{ name, type }] from object literal
  ctx.arg(n).asNumber()    // extract numeric literal
  ctx.arg(n).asIdent()     // extract identifier name
  ctx.read(path)           // read a file from disk at compile time
  ctx.emit(zeusSource)     // splice Zeus source into the call site
  ctx.error(msg)           // abort with a source-located compile error
  ctx.warn(msg)            // emit a compiler warning
}
```

---

## Feature Catalog

### 1. Type-safe JSON from Files

Read a JSON file at compile time. The inferred struct type is known to the compiler and LSP — no `any`, no runtime parsing, no generated code files to commit.

```js
// json.macro.js
export function fromFile(ctx, path) {
  const raw    = ctx.read(path.asString());
  const parsed = JSON.parse(raw);

  function toZeusType(v) {
    if (typeof v === "string")  return "str";
    if (typeof v === "number")  return Number.isInteger(v) ? "i64" : "f64";
    if (typeof v === "boolean") return "bool";
    if (Array.isArray(v))       return `${toZeusType(v[0])}[]`;
    if (typeof v === "object") {
      const fields = Object.entries(v).map(([k, v]) => `${k}: ${toZeusType(v)}`).join(", ");
      return `{ ${fields} }`;
    }
  }

  const fields = Object.entries(parsed)
    .map(([k, v]) => `  ${k}: ${toZeusType(v)}`)
    .join(",\n");

  return ctx.emit(`{ ${fields} }`);
}
```

```zeus
import { fromFile } from "./json.macro.js"

let config = fromFile("./config.json")

// LSP suggests: config.database.host, config.port, config.app.name
// Rename a key in config.json → compile error here, not a runtime crash
println(config.database.host)
```

**Why it matters:** every language has JSON parsing. No language has *compile-time typed* JSON access where a schema change breaks the build immediately.

---

### 2. Type-safe Environment Variables

```zeus
import { fromEnv } from "./env.macro.js"

let env = fromEnv("./.env")

// Missing key in .env → compile error, not runtime panic
println(env.DATABASE_URL)   // str, guaranteed present at compile time
println(env.PORT)           // str, LSP-suggested
```

Ship binaries that literally cannot start with a missing env var — the compiler rejects the build.

---

### 3. ORM / Database Schema

Define tables as Zeus code; get fully-typed CRUD functions with zero boilerplate and zero runtime query builder overhead.

```zeus
import { table } from "./orm.macro.js"

table("users", {
  id:         i64,
  name:       str,
  email:      str,
  created_at: i64
})

fn main(): void {
  let user = users_find_by_id(1)!          // returns UsersRow?
  users_insert(2, "Ameer", "a@b.com", 0)  // wrong arg type → compile error
  let all = users_find_all()               // UsersRow[]
}
```

Or read directly from a SQL schema file:

```zeus
import { fromSQL } from "./schema.macro.js"

fromSQL("./schema.sql")   // parses DDL, generates all structs + queries
```

Every table, every column, every type — known at compile time. Drizzle ORM's guarantee, but enforced by the compiler.

---

### 4. GraphQL — Compile-time Query Validation

```zeus
import { gql } from "./graphql.macro.js"

// validates query against schema.graphql at compile time
// result type is fully inferred from the query shape
let result = gql("./schema.graphql", `
  query {
    user(id: 1) {
      name
      email
      posts { title }
    }
  }
`)

println(result.user.name)          // str
println(result.user.posts[0].title) // str
// result.user.nonexistent         → compile error: field does not exist in schema
```

---

### 5. OpenAPI → Typed HTTP Client

```zeus
import { fromOpenAPI } from "./http.macro.js"

fromOpenAPI("./openapi.yaml")

fn main(): void {
  // every endpoint, every param, every response body — typed
  let user    = GET_users_id(42)!           // returns UserResponse
  let created = POST_users({ name: "Ameer", email: "a@b.com" })
  //            ^^^^^^^^^ wrong body shape → compile error
}
```

No code generation step. No committed generated files that drift from the spec.

---

### 6. Protobuf / Binary Protocol Types

```zeus
import { proto } from "./proto.macro.js"

proto("./user.proto")

fn main(): void {
  let msg    = UserMessage { id: 1, name: "Ameer" }
  let bytes  = msg.encode()   // generated by macro
  let parsed = UserMessage.decode(bytes)!
}
```

Macro reads the `.proto` file, generates Zeus structs + encode/decode functions, with field IDs baked in as constants. Faster than runtime reflection-based serialization.

---

### 7. Compile-time Asset Embedding

```zeus
import { embedFile, embedDir } from "./embed.macro.js"

// byte array literal baked into the binary at compile time
let logo  = embedFile("./assets/logo.png")      // u8[]
let font  = embedFile("./assets/Inter.ttf")     // u8[]

// typed map — LSP suggests valid filenames
let assets = embedDir("./assets/")
let icon   = assets["icon.svg"]                 // u8[]
// assets["missing.png"]                        → compile error: file not found
```

No `go:embed` magic comments. No runtime file I/O paths to get wrong. The binary is fully self-contained and the compiler verifies every asset reference.

---

### 8. I18n — Compile-time Translation Key Verification

```zeus
import { i18n } from "./i18n.macro.js"

i18n("./locales/en.json")

fn main(): void {
  println(t("greeting"))          // valid key — ok
  println(t("user.profile.bio"))  // valid nested key — ok
  println(t("typo.key"))          // → compile error: key not found in en.json
}
```

Every translation key verified at compile time. Add a key to the locale file, get LSP autocomplete for it immediately. Remove a key, find every broken reference before shipping.

---

### 9. Regex with Typed Named Captures

```zeus
import { regex } from "./regex.macro.js"

// macro parses the regex at compile time, compiles it, validates named capture groups
let pat = regex(`(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})`)

let m = pat.match("2026-06-28")!
println(m.year)    // str — LSP knows this field exists
println(m.month)   // str
// println(m.foo)  → compile error: capture group "foo" does not exist
```

Named capture groups become struct fields. Pattern is compiled once. No runtime regex compilation.

---

### 10. Derive Macros (like Rust's `#[derive]`)

```zeus
import { derive } from "./derive.macro.js"

derive("Serialize", "Debug", "Builder")
struct User {
  id:    i64,
  name:  str,
  email: str
}

fn main(): void {
  let user = User.builder()
               .id(1)
               .name("Ameer")
               .email("a@b.com")
               .build()

  println(user.debug())       // "User { id: 1, name: \"Ameer\", ... }"
  let json = user.toJSON()    // "{\"id\":1,\"name\":\"Ameer\",...}"
}
```

Each derive target is a separate `.macro.js` export. User-land, not baked into the compiler.

---

### 11. State Machines

```zeus
import { stateMachine } from "./fsm.macro.js"

stateMachine("Fetch", {
  states:      ["idle", "loading", "success", "error"],
  initial:     "idle",
  transitions: {
    "idle    → loading": "start",
    "loading → success": "resolve",
    "loading → error":   "reject",
    "error   → idle":    "reset",
    "success → idle":    "reset"
  }
})

fn main(): void {
  let fsm = FetchMachine.new()
  fsm.start()     // ok: idle → loading
  fsm.resolve()   // ok: loading → success
  fsm.start()     // → compile error: transition "start" not valid from "success"
}
```

Invalid transitions caught at compile time. The entire state machine is generated as Zeus code — no runtime interpreter, no table dispatch overhead.

---

### 12. Feature Flags — Dead Code Elimination at Compile Time

```js
// flags.macro.js
export function feature(ctx, flag) {
  const flags = JSON.parse(ctx.read("./feature-flags.json"));
  const enabled = flags[flag.asString()] ?? false;
  return ctx.emit(enabled ? "true" : "false");
}
```

```zeus
import { feature } from "./flags.macro.js"

if feature("new-dashboard") {
  showNewDashboard()    // this branch is compiled out entirely when flag is false
} else {
  showLegacyDashboard()
}
```

Ships zero bytes for disabled features. Toggle a flag in JSON, recompile — no code changes, no `#ifdef` mess.

---

### 13. Compile-time Lookup Tables

```zeus
import { sinTable, lerpTable } from "./math.macro.js"

// pre-computed at compile time — JS Math.sin runs during compilation
let SIN_TABLE = sinTable(256)       // f64[256], baked into binary
let val       = SIN_TABLE[angle]    // pure array lookup at runtime
```

Arbitrary JS math runs at compile time to produce constant arrays. No runtime computation cost.

---

### 14. CSS Modules — Typed Class Names

```zeus
import { css } from "./css.macro.js"

let styles = css("./Button.module.css")

// LSP suggests: styles.button, styles.label, styles.active
// Rename a class in the CSS file → compile error here
element.className = styles.button
// element.className = styles.typo  → compile error: class not found
```

---

### 15. File-based Routing (SvelteKit-style)

```zeus
import { routes } from "./router.macro.js"

// scans ./handlers/ directory at compile time
// generates typed dispatch table from file names
let router = routes("./handlers/")

// handlers/users.zs, handlers/posts.zs → typed route constants
router.dispatch(req, router.USERS)    // compile error if route doesn't exist
```

---

### 16. Enum from External Data Source

```zeus
import { enumFromCSV, enumFromJSON } from "./enum.macro.js"

// reads countries.csv at compile time, generates Zeus enum
enumFromCSV("./data/countries.csv", column: "code")

fn main(): void {
  let c = CountryCode.US    // LSP suggests all values
  // CountryCode.XYZ        → compile error: not a valid country code
}
```

Enums that stay in sync with your data files automatically. Add a row to the CSV — new enum value appears. Delete one — every reference breaks at compile time.

---

## What Macros Cannot Do (and What Fills the Gap)

| Goal | Macro alone? | Native feature needed |
|---|---|---|
| `{ key: val }` object literal syntax | No — parser change needed | Anonymous struct types |
| LSP completions for basic literals | No | Native anonymous struct support in type checker |
| LSP completions for macro results | Partial — needs macro expansion cache in LSP | Macro-aware LSP protocol |
| Change operator precedence | No | Parser change |
| New control flow syntax | No | Parser + IR change |

The pattern: **macros own the semantics, native language changes own the syntax**. Macros that read external files and generate typed Zeus code are purely additive — they never need parser changes. Macros that want to introduce new *expression forms* (`{}` literals, tagged templates) need corresponding parser support.

---

## Implementation Order

1. **QuickJS embedding** — CGo shim, `ctx` API, `.macro.js` import detection
2. **`ctx.read()` + `ctx.emit()` + `ctx.error()`** — the minimal viable macro system
3. **`ctx.arg(n).asX()`** — typed AST argument extraction
4. **ORM + JSON macros** — first real user-facing features, prove the system
5. **Anonymous struct types** (native) — unlocks object literal syntax + basic LSP
6. **Macro expansion cache in LSP** — LSP completions for macro-generated names

Steps 1–4 are a self-contained unit. Steps 5–6 are independent language features that compound the value.
