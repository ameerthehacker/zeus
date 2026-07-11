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
