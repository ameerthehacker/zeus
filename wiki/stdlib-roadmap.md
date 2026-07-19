# Zeus standard library — Node parity roadmap

Living tracker of Zeus's stdlib versus Node's **synchronous** surface. Zeus has no async/await,
generics, or regex engine yet (see `wiki/` + the async/generics notes), so the reachable target is
Node's blocking API. Legend: **✅ have** · **🟡 partial** · **⛔ blocked** (needs a language feature) ·
**▫️ todo**.

## How the stdlib is wired (for contributors)

- **`@std/X` modules** — `lib/std/X.zs`, resolved from `$ZEUS_HOME/lib/std/` by
  `internal/module/module.go`. Ordinary Zeus modules: real classes/methods, may call libc
  (`@extern("C", ...)`) and the ambient C-FFI primitives in `internal/prelude/c_ffi.zs`.
- **Primordials** — `internal/prelude/*.zs` (globals/built-in types: `string`, `Math`, `console`,
  `process`, timers, number parsers). Their class methods must be `@extern` (bodies are runtime
  functions); free functions bind directly.
- **Runtime** — `runtime/*.zig` (`@extern` targets). Method fat-ABI (`this, return_buffer, ...args`)
  vs direct C ABI (`@extern("zeus","name")`, args/result by value). New files must be referenced in
  `runtime/main.zig`. Zig's `std.crypto`/`std.time`/`std.base64` are available with no external deps.

## String (JS methods on `string`)

✅ `length`, `compare`, `equals`, `concat`, `toString`, indexing, `slice`, `substring`, `indexOf`,
`lastIndexOf`, `includes`, `startsWith`, `endsWith`, `toUpperCase`, `toLowerCase`, `trim`/`trimStart`/
`trimEnd`, `repeat`, `padStart`/`padEnd`, `replace`/`replaceAll` (literal), `charAt`, `charCodeAt`,
`split`.
▫️ `substr` (deprecated), `at`, `codePointAt`, `normalize`, `localeCompare`.
⛔ `match`, `matchAll`, regex `replace`/`split` — need a **regex engine**.

## @std/path  ✅ (POSIX)

`join`, `resolve`, `normalize`, `dirname`, `basename`, `extname`, `isAbsolute`, `relative`,
`parse`/`format` (+ `ParsedPath`), `sep()`, `delimiter()`. Variadic `join`/`resolve` take a `string[]`
(Zeus free functions aren't variadic). Windows paths are out of scope (Zeus targets macOS/Linux).

## @std/fs

✅ `readFileSync`, `writeFileSync`, `appendFileSync`, `copyFileSync`, `existsSync`, `accessSync`,
`statSync`/`lstatSync` (+ `Stats`), `realpathSync`, `readdirSync`, `readdirTypesSync` (+ `Dirent`),
`mkdirSync`, `mkdirpSync` (recursive), `rmSync` (with recursive flag), `rmdirSync`, `unlinkSync`,
`renameSync`, `chmodSync`, `truncateSync`.
▫️ fd-based `openSync`/`readSync`/`writeSync`/`closeSync`/`fsyncSync`, `symlinkSync`/`readlinkSync`/
`linkSync`, `utimesSync`, `mkdtempSync`, `fs.constants`, `watch`.
⛔ `fs.promises` — needs **async**.

## @std/os

✅ `platform`, `arch`, `hostname`, `homedir`, `tmpdir`, `totalmem`, `freemem`, `availableParallelism`,
`type`, `release`, `version`, `machine`, `endianness`, `EOL`, `loadavg` (+ `LoadAvg`).
▫️ `cpus()` (per-core model/speed array), `uptime`, `userInfo`, `networkInterfaces`, `constants`.

## process (global)

✅ `cwd`, `chdir`, `getEnv`/`setEnv`/`hasEnv`, `pid`, `ppid`, `exit`, `argv`, `execPath`, `hrtime`
(ns), `platform`, `arch`.
▫️ `env` as an object/iteration, `uptime`, `memoryUsage`, `nextTick`, `stdout`/`stderr`/`stdin`
streams, `kill`, `umask`, `exitCode` property, `versions`.

## Math (global)

✅ full set: sqrt/cbrt/pow/exp/log/log2/log10, sin/cos/tan, asin/acos/atan/atan2, sinh/cosh/tanh,
asinh/acosh/atanh, floor/ceil/round/trunc, abs/sign/min/max/hypot, log1p/expm1, clz32/fround/imul,
random, and constants PI/E/LN2/LN10/LOG2E/LOG10E/SQRT2/SQRT1_2.

## Numbers / globals

✅ `parseInt`, `parseFloat`, `isNaN`, `isFinite` (global, JS-lenient, return f64).
▫️ `Number.toFixed`/`toPrecision`/`toString(radix)`, `Number.isInteger`/`isSafeInteger`,
`encodeURIComponent`/`decodeURIComponent`, `btoa`/`atob`, `structuredClone`.

## @std/datetime  ✅

`Date` (built from epoch ms — no overloaded ctors): static `now()`, `getTime`, `getFullYear`,
`getMonth` (0-based), `getDate`, `getHours`, `getMinutes`, `getSeconds`, `getMilliseconds`, `getDay`,
`getTimezoneOffset`, `toISOString`. getX() are local-time; toISOString() is UTC.
▫️ UTC getters (`getUTCHours`…), setters, `Date.parse`/ISO-string ctor, `toLocaleString`.

## console (global)

✅ `log`, `info`, `debug`, `warn`, `error` (variadic, reflective stringify).
▫️ `table`, `dir`, `time`/`timeEnd`, `count`, `group`/`groupEnd`, `trace`, `assert`, `clear`.

## JSON  ✅ (global, like TS)

Global `JSON` + `JsonValue` primordials (no import, like `console`/`Math`): `JSON.parse`/`stringify`
and `JSON.new*` builders; JsonValue accessors (`get`/`at`/`asString`/`asInt`/`kind`/`objectKeys`/…).
The document is a tree of GC-allocated nodes in the runtime (`json_runtime.zig`), traced through the
`JsonValue.node` field. ⛔ `JSON.stringify(nativeObject)` — needs an `any` type (build a JsonValue).

**Planned ergonomic upgrade** (⛔ needs generics + union/structural types): the explicit `get`/
`asString` tree API is a stopgap. The target is a JS-like typed value —
`let v: JSONValue<{ name: string }> = JSON.parse(str); v.name` (or `v["name"]`) — with dot/bracket
field access typed by the structural parameter. Tracked in the root `roadmap.md` under JSON objects.

## @std/buffer + encoding

`Buffer` over `u8[]` (`from`/`alloc`/`toString('utf8'|'hex'|'base64')`/read+write ints/`concat`/
`slice`), base64/hex encode/decode.

## @std/crypto — Phase 8 (Zig `std.crypto`, no OpenSSL)

Hashes (md5/sha1/sha224/256/384/512), HMAC, `randomBytes`/`randomUUID`/`randomInt`, `pbkdf2Sync`,
`scryptSync`, `timingSafeEqual`. Follow-on (still Zig-native): AEAD `createCipheriv`/`createDecipheriv`
(aes-gcm/chacha20-poly1305), Ed25519 sign/verify, X25519 ECDH.
⛔ RSA, AES-CBC-PKCS7, classic DH, X.509 — not in Zig std; need a vendored lib.

## Blocked on a language feature (catalogued, not attempted)

- **Generics**: `Map`, `Set`, `Array` higher-order (`map`/`filter`/`reduce`/`forEach`/`find`/`sort`/
  `join`/`slice`/`splice`), `EventEmitter`, typed containers.
- **Regex engine**: `RegExp`, `String.match`, regex `replace`/`split`.
- **Async/await**: `fs.promises`, `net`/`http`/`dns` clients, streams, `child_process` async,
  `setImmediate`/`queueMicrotask`. (A raw event-loop `@std/http` already exists.)
- **fd/stream I/O**: `process.stdout`/`stdin` as streams, `string_decoder`.
