// Package prelude holds Zeus-source definitions of primordials ("preludes") that are compiled and
// registered as built-in classes/functions when the IR builder first loads, instead of being
// hand-built in Go (see ir.loadPreludes). Self-hosted so far: Console, Error, string, and the timer
// functions. Arrays and ref cells stay Go-built because they are generic (Zeus has no generics yet).
//
// Adding a prelude is drop-in: put a `.zs` file in this directory and it is embedded and loaded
// automatically — no Go changes for a plain prelude. (`string` is loaded first because others may
// reference it; a class that needs a reserved class ID is listed in ir.reservedClassIds.)
package prelude

import "embed"

// FS embeds every prelude .zs source in this directory.
//
//go:embed *.zs
var FS embed.FS

// GlobalsFile is the prelude that declares ambient `global`s (console, Math). Unlike the other
// preludes it defines no classes — it constructs the singleton instances — so it is NOT harvested
// as a class prelude (see ir.loadPreludes); instead the compiler injects it as a real, always-linked
// module compiled first, so its init runs before any user code.
const GlobalsFile = "globals.zs"

// GlobalsModulePath is the synthetic module path of the injected ambient-globals prelude. It is the
// owning module recorded for console/Math in the ambient-global registry, so the compiler's injected
// globals.zs (which carries this same path) is recognized as the definer rather than a duplicate.
const GlobalsModulePath = "<zeus-prelude-globals>"
