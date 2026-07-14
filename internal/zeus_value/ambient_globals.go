package zeus_value

import (
	"fmt"
	"sync"

	"github.com/ameerthehacker/zeus/internal/token"
)

// AmbientGlobalInfo records a program-wide `global` definition: which module owns it,
// its declared type, and where it was declared (for duplicate-error reporting).
type AmbientGlobalInfo struct {
	Name               string
	DefiningModulePath string
	ValueType          ValueType
	Span               *token.Span
}

// ambientGlobals is the program-wide registry of `global` declarations. A `global` is an ambient
// symbol: visible in every module without import, backed by a single external-linkage storage cell
// (see AmbientGlobalSymbolName).
//
// It has two tiers:
//   - prelude: console/Math, seeded once from globals.zs when preludes load. Never reset, so they
//     resolve on EVERY compile path — including the language server, which type-checks a single
//     document without injecting/compiling globals.zs.
//   - user: everything a user program declares. Reset per compilation (ResetAmbientGlobals) because
//     the LSP/tests reuse the process.
//
// All lookups consult both; duplicate detection spans both.
var ambientGlobals = struct {
	mu      sync.RWMutex
	prelude map[string]*AmbientGlobalInfo
	user    map[string]*AmbientGlobalInfo
}{
	prelude: make(map[string]*AmbientGlobalInfo),
	user:    make(map[string]*AmbientGlobalInfo),
}

// lookupLocked returns the entry for name from either tier (user shadows prelude), caller holds mu.
func lookupLocked(name string) *AmbientGlobalInfo {
	if info, ok := ambientGlobals.user[name]; ok {
		return info
	}
	if info, ok := ambientGlobals.prelude[name]; ok {
		return info
	}
	return nil
}

// ResetAmbientGlobals clears user ambient-global registrations (not the never-reset prelude tier).
// Call at the start of the IR-generation phase so a fresh compilation does not see stale entries.
func ResetAmbientGlobals() {
	ambientGlobals.mu.Lock()
	defer ambientGlobals.mu.Unlock()
	ambientGlobals.user = make(map[string]*AmbientGlobalInfo)
}

// FreezeUserAmbientGlobalsAsPrelude promotes every current user entry into the never-reset prelude
// tier and clears the user tier. Used once when preludes load: globals.zs is compiled to resolve
// console/Math (with concrete object types), and the result is frozen so it survives resets.
func FreezeUserAmbientGlobalsAsPrelude() {
	ambientGlobals.mu.Lock()
	defer ambientGlobals.mu.Unlock()
	for name, info := range ambientGlobals.user {
		ambientGlobals.prelude[name] = info
	}
	ambientGlobals.user = make(map[string]*AmbientGlobalInfo)
}

// RegisterAmbientGlobalDef records a `global` definition. Declaring the same name in a *different*
// module is a duplicate-definition error. Re-registration from the same module (across passes) is
// idempotent and refreshes the recorded type/span.
func RegisterAmbientGlobalDef(name string, modulePath string, valueType ValueType, span *token.Span) error {
	ambientGlobals.mu.Lock()
	defer ambientGlobals.mu.Unlock()

	if existing := lookupLocked(name); existing != nil {
		if existing.DefiningModulePath != modulePath {
			return fmt.Errorf("global '%s' is already defined in module '%s'", name, existing.DefiningModulePath)
		}
		existing.ValueType = valueType
		existing.Span = span
		return nil
	}

	ambientGlobals.user[name] = &AmbientGlobalInfo{
		Name:               name,
		DefiningModulePath: modulePath,
		ValueType:          valueType,
		Span:               span,
	}
	return nil
}

// RefineAmbientGlobalType updates the recorded type of a global owned by modulePath — used when the
// defining module's IR generation resolves a more precise type than the pre-scan inferred. It does
// no duplicate checking (the pre-scan owns that); if the global is not yet registered, it registers
// it in the user tier.
func RefineAmbientGlobalType(name string, modulePath string, valueType ValueType, span *token.Span) {
	ambientGlobals.mu.Lock()
	defer ambientGlobals.mu.Unlock()

	if existing := lookupLocked(name); existing != nil {
		if existing.DefiningModulePath == modulePath {
			existing.ValueType = valueType
		}
		return
	}
	ambientGlobals.user[name] = &AmbientGlobalInfo{
		Name:               name,
		DefiningModulePath: modulePath,
		ValueType:          valueType,
		Span:               span,
	}
}

// LookupAmbientGlobal returns the registered info for an ambient global, if any.
func LookupAmbientGlobal(name string) (*AmbientGlobalInfo, bool) {
	ambientGlobals.mu.RLock()
	defer ambientGlobals.mu.RUnlock()
	info := lookupLocked(name)
	return info, info != nil
}

// IsAmbientGlobal reports whether name is a registered ambient global.
func IsAmbientGlobal(name string) bool {
	ambientGlobals.mu.RLock()
	defer ambientGlobals.mu.RUnlock()
	return lookupLocked(name) != nil
}

// AllAmbientGlobals returns a snapshot of every registered ambient global (both tiers).
func AllAmbientGlobals() []*AmbientGlobalInfo {
	ambientGlobals.mu.RLock()
	defer ambientGlobals.mu.RUnlock()
	out := make([]*AmbientGlobalInfo, 0, len(ambientGlobals.prelude)+len(ambientGlobals.user))
	for _, info := range ambientGlobals.prelude {
		if _, shadowed := ambientGlobals.user[info.Name]; !shadowed {
			out = append(out, info)
		}
	}
	for _, info := range ambientGlobals.user {
		out = append(out, info)
	}
	return out
}

// AmbientGlobalSymbolName is the stable, un-mangled LLVM symbol shared by a `global`'s single
// definition and every module's extern reference to it. Not user-producible, so it cannot collide
// with a source-derived name.
func AmbientGlobalSymbolName(name string) string {
	return "zeus_global_" + name
}
