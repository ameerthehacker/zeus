package ir

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/prelude"
	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type IRBuilder struct {
	instrs                  []*Instr
	currentBlock            *BasicBlock
	insertionIndex          int
	blockIdInsetionIndexMap map[int]int
	blocks                  []*BasicBlock
	tempVarCount            int
	blocksCount             int
	symbolTable             *symbol_table.SymbolTable[zeus_value.Value]
	instrIdCount            int
	usedFuncIRNames         map[string]bool
	usedVarNames            map[string]bool
	varNameScopeStack       []map[string]bool
	usedGlobalNames         map[string]bool
}

func NewIRBuilder() *IRBuilder {
	// Preludes are compiled once (lazily) before the first builder is populated, so the primordial
	// classes they define are in the registry for every IR-gen — the real compiler and unit tests
	// (which construct IRBuilders directly, bypassing the compiler) alike.
	ensurePreludesLoaded()
	return newIRBuilderInternal()
}

// newIRBuilderInternal creates a builder and registers the current registry primordials WITHOUT
// triggering prelude loading. Used by NewIRBuilder (after preludes are loaded) and by the prelude
// compilation itself, so compiling a prelude does not re-enter ensurePreludesLoaded (no recursion).
func newIRBuilderInternal() *IRBuilder {
	symbol_table := symbol_table.NewSymbolTable[zeus_value.Value]()
	symbol_table.EnterScope()

	builder := &IRBuilder{
		currentBlock:            nil,
		tempVarCount:            0,
		blocksCount:             0,
		insertionIndex:          0,
		instrIdCount:            0,
		blockIdInsetionIndexMap: make(map[int]int),
		symbolTable:             symbol_table,
		usedFuncIRNames:         make(map[string]bool),
		usedVarNames:            make(map[string]bool),
		usedGlobalNames:         make(map[string]bool),
	}

	// Register all primordial classes from the registry upfront
	// This ensures DECL_CLASS instructions are always at the beginning
	builder.initializePrimordials()

	return builder
}

var preludeOnce sync.Once

// ensurePreludesLoaded compiles the embedded prelude sources once and registers the resulting
// primordial classes/functions. Runs before any builder is populated.
func ensurePreludesLoaded() {
	preludeOnce.Do(loadPreludes)
}

// EnsurePreludesLoaded compiles the embedded preludes (once, process-wide) so primordial
// classes/functions and the prelude ambient globals (console/Math) are registered. The compiler
// calls this before its ambient-global pre-scan so the prelude tier is seeded first (loadPreludes
// resets the user tier when it freezes console/Math). Safe to call repeatedly.
func EnsurePreludesLoaded() {
	ensurePreludesLoaded()
}

// reservedClassIds pins a fixed class ID for prelude classes that need one — Error's ID backs O(1)
// exception-type matching (IsErrorClass). Add an entry to reserve an ID for a new prelude class.
var reservedClassIds = map[string]int{"Error": zeus_value.ERROR_CLASS_ID}

// loadPreludes discovers every embedded prelude .zs, compiles it, and registers the classes it
// declares (extern free functions self-register during compile). `string` is loaded first because
// other preludes reference it. A failure here is a compiler/prelude bug, not user input, so it panics.
func loadPreludes() {
	entries, err := prelude.FS.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("reading preludes: %s", err))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".zs") {
			names = append(names, e.Name())
		}
	}
	// `string` is the foundational primordial the others reference, so load it first; the rest are
	// order-independent and loaded alphabetically for determinism.
	sort.Slice(names, func(i, j int) bool {
		const first = "string.zs"
		if names[i] == first || names[j] == first {
			return names[i] == first
		}
		return names[i] < names[j]
	})

	for _, name := range names {
		// globals.zs declares ambient globals, not classes; the compiler injects it as a real
		// module instead of harvesting classes from it here.
		if name == prelude.GlobalsFile {
			continue
		}
		src, err := prelude.FS.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("reading prelude %s: %s", name, err))
		}
		builder, err := compilePrelude(string(src))
		if err != nil {
			panic(fmt.Sprintf("failed to load prelude %s: %s", name, err))
		}
		// A prelude's own class is the DECL_CLASS with an empty PrimordialName. The primordials that
		// initializePrimordials re-emits into this throwaway builder (arrays + already-loaded
		// preludes) all carry a non-empty tag, so this cleanly isolates what THIS file declares.
		for _, instr := range builder.GetInstrs() {
			if instr.Type != InstrTypeDeclClass {
				continue
			}
			class := AsDeclClassInstrInput(instr.Input).Class
			if class.PrimordialName != "" {
				continue
			}
			// Any non-empty tag keeps the class on the internal-linkage + codegen factory path; the
			// specific value is not read (only ARRAY and "" are special-cased elsewhere), so the
			// class name serves. A reserved ID is applied when one is registered for this class.
			class.PrimordialName = class.SourceName()
			if id, ok := reservedClassIds[class.SourceName()]; ok {
				class.Id = id
			}
			zeus_value.Registry.RegisterClass(class)
		}

		// Interfaces emit no IR (VisitInterfaceDeclExpr only declares them in the symbol table), so
		// harvest them from the compiled prelude's symbol table. Register only interfaces not already
		// registered, so a primordial injected into a later prelude's symbol table isn't re-added.
		builder.symbolTable.Walk(func(_ string, value zeus_value.Value) {
			if iface := zeus_value.AsInterface(value); iface != nil && zeus_value.Registry.GetInterface(iface.Name) == nil {
				zeus_value.Registry.RegisterInterface(iface)
			}
		})
	}

	// Seed the ambient-global registry with globals.zs's console/Math (resolved to concrete object
	// types) into the never-reset prelude tier, so they resolve on every compile path — including the
	// language server, which type-checks a single document and never injects/compiles globals.zs.
	loadPreludeGlobals()
}

// loadPreludeGlobals compiles globals.zs (which declares `global console`) so the singleton's
// object type is resolved, then freezes those registrations into the never-reset prelude tier of
// the ambient-global registry. Runs after the class preludes so Console is in the registry when
// `new Console()` is evaluated. (Math is a pure static class with no singleton.)
func loadPreludeGlobals() {
	src, err := prelude.FS.ReadFile(prelude.GlobalsFile)
	if err != nil {
		panic(fmt.Sprintf("reading prelude %s: %s", prelude.GlobalsFile, err))
	}
	tokens, lexErrs := lexer.NewLexer(string(src)).Lex()
	if len(lexErrs) > 0 {
		panic(fmt.Sprintf("lexing prelude %s: %v", prelude.GlobalsFile, lexErrs))
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		panic(fmt.Sprintf("parsing prelude %s: %v", prelude.GlobalsFile, parseErrs))
	}
	// Pre-scan (register names) then IR-gen (refine to concrete initializer types), mirroring the
	// real compiler, then freeze into the never-reset prelude tier.
	PrescanAmbientGlobals(program, prelude.GlobalsModulePath)
	mod := NewIRModule(newIRBuilderInternal(), prelude.GlobalsModulePath, false, nil)
	mod.Generate(program)
	zeus_value.FreezeUserAmbientGlobalsAsPrelude()

	// All primordial classes/interfaces now have their ids; pin the watermark so a later
	// ResetToPrimordialIds (per-analysis id rewind) never reissues a primordial id.
	zeus_value.SnapshotPrimordialIds()
}

// compilePrelude lexes, parses, and IR-generates a prelude source, returning the populated builder.
// Uses newIRBuilderInternal so it does not re-enter prelude loading. Extern free functions in the
// source self-register into the registry during IR-gen (VisitFunctionDeclExpr).
func compilePrelude(source string) (*IRBuilder, error) {
	l := lexer.NewLexer(source)
	tokens, lexErrs := l.Lex()
	if len(lexErrs) > 0 {
		return nil, fmt.Errorf("lex: %v", lexErrs)
	}
	program, parseErrs := parser.NewParser(tokens).ParseProgram()
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("parse: %v", parseErrs)
	}
	builder := newIRBuilderInternal()
	mod := NewIRModule(builder, "<prelude>", false, nil)
	for _, e := range mod.Generate(program) {
		if e.Severity == zeus_error.ErrorSeverityError {
			return nil, fmt.Errorf("irgen: %s", e.Message)
		}
	}
	return builder, nil
}

// initializePrimordials registers all primordial classes and functions from the registry.
// This is called once during IRBuilder creation to ensure all primordials are declared
// at the start of the instruction list.
func (b *IRBuilder) initializePrimordials() {
	registry := zeus_value.Registry

	// Emit DECL_CLASS for all registered primordial classes
	for _, class := range registry.GetAllClasses() {
		b.emitPrimordialClassDecl(class)
	}

	// Register primordial functions in symbol table (DECL_PRIMORDIAL_FUNC is emitted in Generate)
	for _, fn := range registry.GetAllFunctions() {
		b.symbolTable.DeclareGlobalSymbol(fn.Name, fn)
	}

	// Inject primordial interfaces harvested from preludes (e.g. the umbrella `Number`) so
	// `let n: Number` resolves in every module. Interfaces are type-level only (no IR).
	for _, iface := range registry.GetAllInterfaces() {
		b.symbolTable.DeclareGlobalSymbol(iface.Name, iface)
	}
}

// emitPrimordialClassDecl emits a DECL_CLASS instruction for a primordial class
// and registers it in the symbol table
func (b *IRBuilder) emitPrimordialClassDecl(class *zeus_value.Class) {
	// Register in symbol table
	b.symbolTable.DeclareGlobalSymbol(class.Name, class)

	// Emit DECL_CLASS instruction
	result := b.createTempVariable(class.GetSpan())
	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   class.GetSpan(),
	})
}

// EmitClassDeclAtStart emits a DECL_CLASS instruction at the start of the instruction list.
// This is used for dynamically discovered array classes that need to be declared before use.
func (b *IRBuilder) EmitClassDeclAtStart(class *zeus_value.Class) {
	// Save current state
	savedBlock := b.currentBlock
	savedIndex := b.insertionIndex

	// Switch to global insertion at the start
	b.currentBlock = nil
	b.insertionIndex = 0

	// Emit DECL_CLASS
	result := b.createTempVariable(class.GetSpan())
	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   class.GetSpan(),
	})

	// Restore state (account for inserted instruction)
	b.currentBlock = savedBlock
	b.insertionIndex = savedIndex + 1
}

// generateUniqueGlobalName returns a name not yet present in the global registry.
// Used to generate unique IR-level names for globally visible symbols (functions, classes).
func (b *IRBuilder) generateUniqueGlobalName(name string) string {
	unique_name := name
	count := 1

	for {
		if _, ok := b.symbolTable.GetSymbol(unique_name); !ok {
			break
		}
		unique_name = name + strconv.Itoa(count)
		count++
	}

	return unique_name
}

// uniqueIRName returns an IR name (suffixing with 1, 2, ... as needed) that is neither currently
// visible in the symbol table nor already present in `used`, and records it in `used`. The live
// symbol-table check catches names minted elsewhere but visible here; `used` additionally catches
// names from already-exited scopes that the live check can no longer see. Callers pick which `used`
// registry (and thus which lifetime) applies.
func (b *IRBuilder) uniqueIRName(name string, used map[string]bool) string {
	unique := name
	for count := 1; ; count++ {
		_, inScope := b.symbolTable.GetSymbol(unique)
		if !inScope && !used[unique] {
			break
		}
		unique = name + strconv.Itoa(count)
	}
	used[unique] = true
	return unique
}

// generateUniqueFuncIRName returns a function IR name that is unique across the entire module.
// Unlike generateUniqueGlobalName, it also checks usedFuncIRNames so names from already-exited
// scopes are still avoided (live symbol-table check alone misses them).
func (b *IRBuilder) generateUniqueFuncIRName(name string) string {
	return b.uniqueIRName(name, b.usedFuncIRNames)
}

// generateUniqueVarIRName returns a variable IR name that is unique within the current function.
// It combines two checks that answer different questions, because IR gen (unlike LLVM IR) has real
// lexical scopes:
//   - the live symbol table catches names that are VISIBLE here but were minted elsewhere — params,
//     module globals, enclosing-scope locals — all of which share codegen's one name-keyed value map
//     (c.llvmValues) with this local;
//   - usedVarNames catches names from already-EXITED scopes, which the live symbol table can no longer
//     see. Two locals with the same source name in sibling (non-overlapping) scopes — e.g. `let n` in
//     two different `if` branches — must NOT share an IR name, or the two allocas collide in
//     c.llvmValues and a later branch's load resolves to the wrong (uninitialized) slot, producing
//     malformed LLVM IR that crashes object emission.
//
// usedVarNames is scoped per function (see Begin/EndFunctionNameScope): var IR names only need to be
// unique within a function because codegen processes functions sequentially and rebinds its value map
// at each entry, so reusing a plain name across functions is safe and keeps the IR readable.
func (b *IRBuilder) generateUniqueVarIRName(name string) string {
	return b.uniqueIRName(name, b.usedVarNames)
}

// BeginFunctionNameScope pushes a fresh per-function variable-name registry (the current one is saved
// on an internal stack). Wrap a function body's IR gen in a BeginFunctionNameScope/EndFunctionNameScope
// pair so nested functions (closures/methods) get their own registry and same-named locals in different
// functions keep their plain IR names instead of accumulating cross-function suffixes. Mirrors the
// symbol table's EnterScope/ExitScope: the builder owns the saved state, the caller passes nothing back.
func (b *IRBuilder) BeginFunctionNameScope() {
	b.varNameScopeStack = append(b.varNameScopeStack, b.usedVarNames)
	b.usedVarNames = make(map[string]bool)
}

// EndFunctionNameScope pops the registry saved by the matching BeginFunctionNameScope.
func (b *IRBuilder) EndFunctionNameScope() {
	last := len(b.varNameScopeStack) - 1
	zeus_error.Assert(last >= 0, "EndFunctionNameScope without a matching BeginFunctionNameScope")
	b.usedVarNames = b.varNameScopeStack[last]
	b.varNameScopeStack = b.varNameScopeStack[:last]
}

func (b *IRBuilder) createTempVariable(span *token.Span) *zeus_value.Var {
	temp_variable_name := zeus_value.TEMP_VARIABLE_PREFIX + strconv.Itoa(b.tempVarCount)
	b.tempVarCount++

	return zeus_value.NewVar(temp_variable_name, nil, false, span)
}

func (b *IRBuilder) GetInsertionBlock() *BasicBlock {
	return b.currentBlock
}

func (b *IRBuilder) pushInstr(instr *Instr) {
	instr.Id = b.instrIdCount
	b.instrIdCount += 1
	if b.currentBlock == nil {
		b.instrs = append(b.instrs[:b.insertionIndex], append([]*Instr{instr}, b.instrs[b.insertionIndex:]...)...)
		b.insertionIndex++
	} else {
		blockInsertionIndex, ok := b.blockIdInsetionIndexMap[b.currentBlock.Id]
		zeus_error.Assert(ok, "block id not found in block id insertion index map")
		b.currentBlock.Instrs = append(b.currentBlock.Instrs[:blockInsertionIndex], append([]*Instr{instr}, b.currentBlock.Instrs[blockInsertionIndex:]...)...)
		b.blockIdInsetionIndexMap[b.currentBlock.Id]++
	}
}

func (b *IRBuilder) BuildSuccessorBlock() *BasicBlock {
	new_block := b.BuildBasicBlock()

	if b.currentBlock != nil {
		b.currentBlock.Successors = append(b.currentBlock.Successors, new_block)
	}

	return new_block
}

func (b *IRBuilder) BuildBasicBlock() *BasicBlock {
	new_block := NewBasicBlock(b.blocksCount)
	b.blockIdInsetionIndexMap[b.blocksCount] = 0
	b.blocks = append(b.blocks, new_block)
	b.blocksCount++

	return new_block
}

func (b *IRBuilder) SetInsertionBlock(block *BasicBlock) {
	b.currentBlock = block
}

// ResetToGlobalEnd switches to top-level (non-block) insertion at the end of the instruction list.
func (b *IRBuilder) ResetToGlobalEnd() {
	b.currentBlock = nil
	b.insertionIndex = len(b.instrs)
}

// InsertionPoint is a snapshot of the builder's insertion state (current block + index). Take one
// with SaveInsertionPoint and reapply it with RestoreInsertionPoint to temporarily retarget
// insertion (e.g. ResetToGlobalEnd to synthesize a top-level function) and then restore it.
type InsertionPoint struct {
	block *BasicBlock
	index int
}

// SaveInsertionPoint snapshots the current insertion state so it can be restored later.
func (b *IRBuilder) SaveInsertionPoint() InsertionPoint {
	return InsertionPoint{block: b.currentBlock, index: b.insertionIndex}
}

// RestoreInsertionPoint reapplies a snapshot taken by SaveInsertionPoint.
func (b *IRBuilder) RestoreInsertionPoint(ip InsertionPoint) {
	b.currentBlock = ip.block
	b.insertionIndex = ip.index
}

func (b *IRBuilder) SetInsertionAfter(instr *Instr) {
	instrIndex := slices.Index(b.instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in instructions list", instr.String()))
	b.insertionIndex = instrIndex + 1
}

func (b *IRBuilder) SetInsertionBefore(instr *Instr) {
	instrIndex := slices.Index(b.instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in instructions list", instr.String()))
	b.insertionIndex = instrIndex
}

// ResetBlockInsertionToEnd sets the builder to append instructions at the end of block.
func (b *IRBuilder) ResetBlockInsertionToEnd(block *BasicBlock) {
	b.currentBlock = block
	b.blockIdInsetionIndexMap[block.Id] = len(block.Instrs)
}

// SplitBlockBefore splits block at instr: instructions from instr onwards move into a
// freshly-created tail block, which inherits block's successor list.
// block's successor list is cleared (caller is responsible for adding the right edges).
// Returns the tail block, or nil if instr is not in block.
func (b *IRBuilder) SplitBlockBefore(block *BasicBlock, instr *Instr) *BasicBlock {
	instrIdx := slices.Index(block.Instrs, instr)
	if instrIdx == -1 {
		return nil
	}

	tailBlock := b.BuildBasicBlock()

	// Move instructions from instrIdx onwards into tailBlock
	tailBlock.Instrs = make([]*Instr, len(block.Instrs)-instrIdx)
	copy(tailBlock.Instrs, block.Instrs[instrIdx:])
	b.blockIdInsetionIndexMap[tailBlock.Id] = len(tailBlock.Instrs)

	// Trim the original block
	block.Instrs = block.Instrs[:instrIdx]
	b.blockIdInsetionIndexMap[block.Id] = instrIdx

	// tailBlock inherits block's successors; caller will set block's new successors
	tailBlock.Successors = block.Successors
	block.Successors = nil

	return tailBlock
}

func (b *IRBuilder) SetBlockInsertionAfter(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex + 1
}

func (b *IRBuilder) SetBlockInsertionBefore(block *BasicBlock, instr *Instr) {
	instrIndex := slices.Index(block.Instrs, instr)
	zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block instructions list", instr.String()))
	b.SetInsertionBlock(block)
	b.blockIdInsetionIndexMap[block.Id] = instrIndex
}

// SetInsertionBeforeInstr positions the builder to insert just before instr.
// If block is non-nil and contains instr, uses block-scoped insertion;
// otherwise falls back to global-list insertion (for module-level instructions).
func (b *IRBuilder) SetInsertionBeforeInstr(block *BasicBlock, instr *Instr) {
	if block != nil {
		if idx := slices.Index(block.Instrs, instr); idx != -1 {
			b.SetInsertionBlock(block)
			b.blockIdInsetionIndexMap[block.Id] = idx
			return
		}
	}
	idx := slices.Index(b.instrs, instr)
	zeus_error.Assert(idx != -1, fmt.Sprintf("instruction %s not found in any known location", instr.String()))
	b.SetInsertionBlock(nil)
	b.insertionIndex = idx
}

// DeleteInstr removes an instruction from the IR.
// If block is nil, the instruction is removed from the global instruction list.
// If block is provided, the instruction is removed from that block's instruction list.
func (b *IRBuilder) DeleteInstr(block *BasicBlock, instr *Instr) {
	if block == nil {
		// Delete from global instructions
		instrIndex := slices.Index(b.instrs, instr)
		zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in global instructions list", instr.String()))
		b.instrs = slices.Delete(b.instrs, instrIndex, instrIndex+1)
		// Adjust insertion index if needed
		if b.insertionIndex > instrIndex {
			b.insertionIndex--
		}
	} else {
		// Delete from block instructions
		instrIndex := slices.Index(block.Instrs, instr)
		zeus_error.Assert(instrIndex != -1, fmt.Sprintf("instruction %s not found in block %d instructions list", instr.String(), block.Id))
		block.Instrs = slices.Delete(block.Instrs, instrIndex, instrIndex+1)
		// Adjust block insertion index if needed
		if blockIdx, ok := b.blockIdInsetionIndexMap[block.Id]; ok && blockIdx > instrIndex {
			b.blockIdInsetionIndexMap[block.Id]--
		}
	}
}

func (b *IRBuilder) BuildBinaryOp(left, right zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   op,
		Output: result,
		Input:  NewBinaryOpInstrInput(left, right),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildExport(modulePath string, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeExport,
		Input: NewExportInstrInput(modulePath, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildLoad(addr *zeus_value.Var, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(addr.Span)
	// Carry the address's (pointee) type onto the loaded value at IR-gen so that member-access
	// resolution — which runs during IR-gen — has the receiver's type available instead of an
	// untyped temp. tcLoad derives the same type from addr later, so this is idempotent.
	result.ValueType = addr.ValueType

	b.pushInstr(&Instr{
		Type:   InstrTypeLoad,
		Output: result,
		Input:  NewLoadInstrInput(addr),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildVarDecl(v *VarDecl) *zeus_value.Var {
	// The IR name (variable.Name) must be unique across the whole function — codegen keys llvmValues
	// by it — so it is uniquified against already-exited scopes too (generateUniqueVarIRName). The
	// symbol table, however, is registered under the SOURCE name so scoped identifier resolution
	// (VisitIdentifier -> GetSymbol(sourceName)) still finds the right binding in each scope. This
	// mirrors how parameters (BuildFuncDecl) and functions register under their source name while
	// carrying a separately-uniquified IR name.
	irName := b.generateUniqueVarIRName(v.Name)

	variable := zeus_value.NewVar(irName, v.ValueType, true, v.Span)
	variable.OriginalName = v.Name
	variable.IsConst = v.IsConst

	b.symbolTable.DeclareSymbol(v.Name, variable)

	b.pushInstr(&Instr{
		Type:  InstrTypeDeclVar,
		Input: NewDeclareVarInstrInput(variable, v.Initializer, v.IsConst),
		Span:  v.Span,
	})

	return variable
}

func (b *IRBuilder) BuildGlobalVarDecl(v *VarDecl) *zeus_value.Var {
	// An ambient `global` uses the stable, un-mangled symbol shared by its single definition and
	// every module's extern reference, so it must NOT be uniquified per module; its symbol-table key
	// is that same stable name. A plain module-scope var, by contrast, gets a module-unique IR name
	// (usedGlobalNames is module-lifetime because globals all coexist, unlike per-function locals) while
	// its symbol is registered under the SOURCE name for scoped resolution — the exact same reasoning as
	// BuildVarDecl. Without this, two same-named module-scope locals in sibling scopes (e.g. `let n` in
	// two module-level `if` branches) share one IR name and collide in codegen's name-keyed value map.
	var irName, symbolKey string
	if v.IsAmbient {
		irName = zeus_value.AmbientGlobalSymbolName(v.Name)
		symbolKey = irName
	} else {
		irName = b.uniqueIRName(v.Name, b.usedGlobalNames)
		symbolKey = v.Name
	}

	variable := zeus_value.NewVar(irName, v.ValueType, true, v.Span)
	variable.OriginalName = v.Name
	variable.IsConst = v.IsConst
	variable.IsAmbient = v.IsAmbient

	b.symbolTable.DeclareSymbol(symbolKey, variable)

	b.pushInstr(&Instr{
		Type:  InstrTypeDeclGlobalVar,
		Input: NewDeclareVarInstrInput(variable, v.Initializer, v.IsConst),
		Span:  v.Span,
	})

	return variable
}

// BuildExternGlobalVarDecl declares a module-local *reference* to an ambient global defined in
// another module: an external declaration (no initializer/storage) under the shared stable symbol.
// The returned Var should be registered in the symbol table under the global's source name so
// ordinary identifier resolution finds it.
func (b *IRBuilder) BuildExternGlobalVarDecl(sourceName string, valueType zeus_value.ValueType, isConst bool, span *token.Span) *zeus_value.Var {
	name := zeus_value.AmbientGlobalSymbolName(sourceName)

	variable := zeus_value.NewVar(name, valueType, true, span)
	variable.OriginalName = sourceName
	variable.IsConst = isConst
	variable.IsExtern = true
	variable.IsUsed = true // referenced across modules; never warn "unused"

	b.pushInstr(&Instr{
		Type:  InstrTypeDeclGlobalVar,
		Input: NewDeclareVarInstrInput(variable, nil, isConst),
		Span:  span,
	})

	return variable
}

func (b *IRBuilder) BuildStore(addr *zeus_value.Var, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeStore,
		Input: NewStoreInstrInput(addr, value),
		Span:  span,
	})
}

// BuildInterfacePropGet emits an INTERFACE_PROP_GET: read `propName` through interface value
// `object`. resultType (the interface property's type) is set on the produced value.
func (b *IRBuilder) BuildInterfacePropGet(object zeus_value.Value, iface *zeus_value.Interface, propName string, resultType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = resultType

	b.pushInstr(&Instr{
		Type:   InstrTypeInterfacePropGet,
		Output: result,
		Input:  NewInterfacePropGetInstrInput(object, iface, propName),
		Span:   span,
	})

	return result
}

// BuildInterfacePropSet emits an INTERFACE_PROP_SET: write `value` into `propName` through
// interface value `object`. No result (the write is a statement).
func (b *IRBuilder) BuildInterfacePropSet(object zeus_value.Value, iface *zeus_value.Interface, propName string, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeInterfacePropSet,
		Input: NewInterfacePropSetInstrInput(object, iface, propName, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCast(value zeus_value.Value, castType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeCast,
		Output: result,
		Input:  NewCastInstrInput(value, castType),
		Span:   span,
	})
	result.ValueType = castType

	return result
}

// BuildInstanceOf emits a runtime type test: result (bool) = value's dynamic class is (a
// subclass of) the class with the given id. Used to guard object `as` downcasts.
func (b *IRBuilder) BuildInstanceOf(value zeus_value.Value, classId int, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeInstanceOf,
		Output: result,
		Input:  NewInstanceOfInstrInput(value, classId),
		Span:   span,
	})
	result.ValueType = zeus_value.BoolType{Span: span}

	return result
}

func (b *IRBuilder) BuildCoerce(value zeus_value.Value, targetType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	b.pushInstr(&Instr{
		Type:   InstrTypeCoerce,
		Output: result,
		Input:  NewCoerceInstrInput(value, targetType),
		Span:   span,
	})
	// Output keeps the source ObjectType, not the target FunctionType — this is the key
	// difference from BuildCast: the variable's type becomes the actual functor ObjectType.
	result.ValueType = zeus_value.GetValueType(value)
	return result
}

func (b *IRBuilder) BuildDeclPrimordialFunc(fn *zeus_value.Function, span *token.Span) {
	b.symbolTable.DeclareGlobalSymbol(fn.Name, fn)
	b.pushInstr(&Instr{
		Type:  InstrTypeDeclPrimordialFunc,
		Input: NewDeclPrimordialFuncInstrInput(fn),
		Span:  span,
	})
}

func (b *IRBuilder) BuildFuncDecl(name string, args []*VarDecl, body *BasicBlock, return_type zeus_value.ValueType, class *zeus_value.Class, span *token.Span) *zeus_value.Function {
	var existingStub *zeus_value.Function
	if existing, ok := b.symbolTable.GetSymbolInCurrentScope(name); ok {
		existingStub = zeus_value.AsFunction(existing)
	}

	b.symbolTable.EnterScope()
	params := []*zeus_value.Var{}
	for _, arg := range args {
		variable := zeus_value.NewVar(b.generateUniqueGlobalName(arg.Name), arg.ValueType, false, arg.Span, arg.IsVariadic)
		b.symbolTable.DeclareSymbol(arg.Name, variable)

		params = append(params, variable)
	}

	// A function is variadic when its final parameter is a rest parameter.
	isVariadic := len(params) > 0 && params[len(params)-1].IsVariadic

	var fn *zeus_value.Function
	if existingStub != nil {
		// Update stub in place so forward-call references remain valid
		existingStub.Params = params
		existingStub.ReturnType = return_type
		existingStub.IsVariadic = isVariadic
		fn = existingStub
		b.usedFuncIRNames[fn.Name] = true
	} else {
		irName := b.generateUniqueFuncIRName(name)
		fn = zeus_value.NewFunction(irName, params, return_type, span)
		fn.IsVariadic = isVariadic
		if irName != name {
			fn.OriginalName = name
		}
	}

	b.symbolTable.ExitScope()
	// Register under the original source name so call-site lookups (GetSymbol(name))
	// resolve correctly regardless of the IR-level unique name.
	if class == nil {
		b.symbolTable.DeclareSymbol(name, fn)
	}

	if class != nil {
		b.pushInstr(&Instr{
			Type:  InstrTypeDeclClassMethod,
			Input: NewDeclClassMethodInstrInput(fn, body, class),
			Span:  span,
		})
	} else {
		b.pushInstr(&Instr{
			Type:  InstrTypeDeclFunc,
			Input: NewDeclFuncInstrInput(fn, body),
			Span:  span,
		})
	}

	return fn
}

func (b *IRBuilder) BuildJmp(target *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeJmp,
		Input: NewJmpInstrInput(target),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCondJmp(true_target *BasicBlock, false_target *BasicBlock, condition zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeCondJmp,
		Input: NewCondJmpInstrInput(true_target, false_target, condition),
		Span:  span,
	})
}

func (b *IRBuilder) BuildCallFunc(callee *zeus_value.Function, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeCallFunc,
		Output: result,
		Input:  NewCallFuncInstrInput(callee, args),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildIndirectFuncCall(callee zeus_value.Value, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeIndirectFuncCall,
		Output: result,
		Input:  NewIndirectFuncCallInstrInput(callee, args),
	})

	return result
}

func (b *IRBuilder) BuildReturn(value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeReturn,
		Input: NewReturnInstrInput(value),
		Span:  span,
	})
}

// BuildCallModuleInit emits a call to a module's init function by its stable external symbol
// name. Codegen declares the symbol extern when its definition lives in another module.
func (b *IRBuilder) BuildCallModuleInit(symbolName string, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeCallModuleInit,
		Input: NewCallModuleInitInstrInput(symbolName),
		Span:  span,
	})
}

func (b *IRBuilder) BuildUnaryOp(value zeus_value.Value, op InstrType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   op,
		Output: result,
		Input:  NewUnaryOpInstrInput(value),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildImport(modulePath string, name string, importedValue zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeImport,
		Input: NewImportInstrInput(modulePath, importedValue),
		Span:  span,
	})

	b.symbolTable.DeclareGlobalSymbol(name, importedValue)
}

func (b *IRBuilder) BuildNewObj(callee zeus_value.Value, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeNewObj,
		Output: result,
		Input:  NewNewObjInstrInput(callee, args),
		Span:   span,
	})

	return result
}

// BuildAllocObj emits an ALLOC_OBJ for the given class. The output is typed as an object of that
// class here (rather than via the type checker) because ALLOC_OBJ is synthesized during lowering,
// after type checking has run.
func (b *IRBuilder) BuildAllocObj(class *zeus_value.Class, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = zeus_value.NewObjectType(class)

	b.pushInstr(&Instr{
		Type:   InstrTypeAllocObj,
		Output: result,
		Input:  NewAllocObjInstrInput(class),
		Span:   span,
	})

	return result
}

// BuildBox emits a BOX autoboxing value into targetClass (Number/Bool). The output is typed as an
// object of that class here (rather than by the type checker) because BOX is emitted while type
// checking is already in progress and the result is used immediately by the consuming instruction.
func (b *IRBuilder) BuildBox(value zeus_value.Value, targetClass *zeus_value.Class, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = zeus_value.NewObjectType(targetClass)

	b.pushInstr(&Instr{
		Type:   InstrTypeBox,
		Output: result,
		Input:  NewBoxInstrInput(value, targetClass),
		Span:   span,
	})

	return result
}

// BuildUnbox emits an UNBOX reading the scalar `value` out of boxValue (a Number/Bool). fieldType is
// the box's field type (f64/boolean) and becomes the output type; the type checker sets it here for
// the same reason as BuildBox (the result is consumed immediately).
func (b *IRBuilder) BuildUnbox(boxValue zeus_value.Value, fieldType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = fieldType

	b.pushInstr(&Instr{
		Type:   InstrTypeUnbox,
		Output: result,
		Input:  NewUnboxInstrInput(boxValue),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildGetIndex(array zeus_value.Value, indices []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeGetIndex,
		Output: result,
		Input:  NewGetIndexInstrInput(array, indices),
		Span:   span,
	})

	return result
}

func (b *IRBuilder) BuildSetIndex(array, index, value zeus_value.Value, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeSetIndex,
		Input: NewSetIndexInstrInput(array, index, value),
		Span:  span,
	})
}

func (b *IRBuilder) BuildClassMethodDecl(method *zeus_value.Function, body *BasicBlock, class *zeus_value.Class, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeDeclClassMethod,
		Input: NewDeclClassMethodInstrInput(method, body, class),
		Span:  span,
	})
}

func (b *IRBuilder) BuildClassDecl(class *zeus_value.Class, span *token.Span) string {
	result := b.createTempVariable(span)

	b.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(class),
		Span:   span,
	})

	return class.Name
}

func (b *IRBuilder) Walk(fnInstr func(instr *Instr), fnBlock func(block *BasicBlock)) {
	worklist := []*BasicBlock{}
	// avoid rewalking the same instruction when new instructions are pushed
	// on top of existing instructions
	visitedInstrs := map[int]bool{}
	i := 0

	for i < len(b.instrs) {
		instr := b.instrs[i]
		_, isVisited := visitedInstrs[instr.Id]
		if isVisited {
			i++
			continue
		}
		visitedInstrs[instr.Id] = true
		fnInstr(instr)

		if IsFunctionDeclInstr(instr.Type) {
			worklist = append(worklist, AsDeclFuncInstrInput(instr.Input).Body)
		}

		if IsClassMethodDeclInstr(instr.Type) {
			worklist = append(worklist, AsDeclClassMethodInstrInput(instr.Input).Body)
		}

		for len(worklist) > 0 {
			block := worklist[0]
			worklist = worklist[1:]
			fnBlock(block)
			j := 0
			// walk the instructions in the block
			for j < len(block.Instrs) {
				instr := block.Instrs[j]
				_, isVisited := visitedInstrs[instr.Id]
				if isVisited {
					j++
					continue
				}
				visitedInstrs[instr.Id] = true
				fnInstr(instr)
				j++
			}
			worklist = append(worklist, block.Successors...)
		}
		i++
	}
}

func (b *IRBuilder) deleteBlock(block *BasicBlock) {
	blockIndex := slices.Index(b.blocks, block)
	if blockIndex == -1 {
		return
	}
	b.blocks = slices.Delete(b.blocks, blockIndex, blockIndex+1)

	// remove this block as successor in other blocks
	for _, otherBlock := range b.blocks {
		otherBlock.Successors = slices.DeleteFunc(otherBlock.Successors, func(successor *BasicBlock) bool {
			return successor.Id == block.Id
		})
	}
}

func (b *IRBuilder) deleteDeadCode(block *BasicBlock) {
	conctrolFlowInstrIndex := slices.IndexFunc(block.Instrs, func(instr *Instr) bool {
		return IsControlFlowInstr(instr.Type)
	})

	// delete all instructions after the control flow instruction
	if conctrolFlowInstrIndex != -1 {
		block.Instrs = slices.Delete(block.Instrs, conctrolFlowInstrIndex+1, len(block.Instrs))
	}
}

func (b *IRBuilder) GetBranchingBlocks(block *BasicBlock) []*BasicBlock {
	branchingBlocks := []*BasicBlock{}

	for _, instr := range block.Instrs {
		switch instr.Type {
		case InstrTypeJmp:
			branchingBlocks = append(branchingBlocks, AsJmpInstrInput(instr.Input).Target)
		case InstrTypeCondJmp:
			branchingBlocks = append(branchingBlocks, AsCondJmpInstrInput(instr.Input).TrueTarget, AsCondJmpInstrInput(instr.Input).FalseTarget)
		case InstrTypePushHandler:
			// PUSH_HANDLER branches to both try body and handler block
			input := AsPushHandlerInstrInput(instr.Input)
			branchingBlocks = append(branchingBlocks, input.TryBodyBlock, input.HandlerBlock)
		case InstrTypeCheckException:
			// CHECK_EXCEPTION branches to handler or continue block
			input := AsCheckExceptionInstrInput(instr.Input)
			branchingBlocks = append(branchingBlocks, input.HandlerBlock, input.ContinueBlock)
		}
	}

	return branchingBlocks
}

func (b *IRBuilder) optimizeBlocks(blocks []*BasicBlock) {
	optimizedBlocks := map[*BasicBlock]bool{}
	var visitAndOptimize func(block *BasicBlock)

	// we delete the dead code in the block and then visit the branching blocks
	visitAndOptimize = func(block *BasicBlock) {
		_, isOptimized := optimizedBlocks[block]
		if isOptimized {
			return
		}

		b.deleteDeadCode(block)

		optimizedBlocks[block] = true
		branchingBlocks := b.GetBranchingBlocks(block)
		for _, branchingBlock := range branchingBlocks {
			visitAndOptimize(branchingBlock)
		}
	}

	for _, block := range blocks {
		visitAndOptimize(block)
	}

	// delete unreachable blocks
	for _, block := range b.blocks {
		_, isOptimized := optimizedBlocks[block]
		if !isOptimized {
			b.deleteBlock(block)
		}
	}
}

func (b *IRBuilder) BuildObjectPropertyAccess(object zeus_value.Value, property string, isLValue bool, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.IsPtr = true

	b.pushInstr(&Instr{
		Type:   InstrTypeObjectPropertyAccess,
		Output: result,
		Input:  NewObjectPropertyAccessInstrInput(object, property, isLValue),
	})

	return result
}

// BuildLoadProperty loads a property value from an object.
// This is a convenience method that combines BuildObjectPropertyAccess and BuildLoad,
// setting the appropriate types on the intermediate variables.
func (b *IRBuilder) BuildLoadProperty(object zeus_value.Value, propertyName string, propertyType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	// Get property pointer
	propPtr := b.BuildObjectPropertyAccess(object, propertyName, false, span)
	propPtrVar := zeus_value.AsVar(propPtr)
	propPtrVar.ValueType = propertyType

	// Load the property value
	propValue := b.BuildLoad(propPtrVar, span)
	propValueVar := zeus_value.AsVar(propValue)
	propValueVar.ValueType = propertyType

	return propValue
}

// BuildMethodCall calls a method on an object, emitting a single CALL_METHOD instruction.
// returnType is set on the result when non-nil (lowering-pass callers supply it;
// TC pass infers it for IR-gen callers where nil is passed).
func (b *IRBuilder) BuildMethodCall(object zeus_value.Value, methodName string, args []zeus_value.Value, returnType zeus_value.ValueType, argTypes []zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	if returnType != nil {
		result.ValueType = returnType
	}

	b.pushInstr(&Instr{
		Type:   InstrTypeMethodCall,
		Output: result,
		Input:  NewMethodCallInstrInput(object, methodName, args),
		Span:   span,
	})

	return result
}

// BuildElemLoad emits a primitive array element read (result = data[index], typed as
// elemType), skipping the array get() method for primitive-element arrays.
func (b *IRBuilder) BuildElemLoad(data, index zeus_value.Value, elemType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = elemType
	b.pushInstr(&Instr{
		Type:   InstrTypeElemLoad,
		Output: result,
		Input:  NewElemLoadInstrInput(data, index, elemType),
		Span:   span,
	})
	return result
}

// BuildElemStore emits a primitive array element write (data[index] = value), skipping
// the array set() method for the in-bounds branch of a primitive-element array write.
func (b *IRBuilder) BuildElemStore(data, index, value zeus_value.Value, elemType zeus_value.ValueType, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeElemStore,
		Input: NewElemStoreInstrInput(data, index, value, elemType),
		Span:  span,
	})
}

// FindBlockContaining returns the basic block that currently holds instr, or nil. Lowering
// passes that split blocks use this: an instruction can move to a freshly-created tail
// block, so the block captured when the instruction was first visited may be stale.
func (b *IRBuilder) FindBlockContaining(instr *Instr) *BasicBlock {
	for _, blk := range b.blocks {
		if slices.Index(blk.Instrs, instr) != -1 {
			return blk
		}
	}
	return nil
}

// BuildSuperConstructorCall emits `super(...)` — a direct call to the base constructor. It has
// no useful result (constructors return void), so the temp output is a void placeholder.
func (b *IRBuilder) BuildSuperConstructorCall(parentClass *zeus_value.Class, thisObject zeus_value.Value, args []zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = zeus_value.VoidType{Span: span}
	b.pushInstr(&Instr{
		Type:   InstrTypeSuperConstructorCall,
		Output: result,
		Input:  NewSuperConstructorCallInstrInput(parentClass, thisObject, args),
		Span:   span,
	})
	return result
}

// BuildStaticMethodCall emits a non-virtual method call (super.method()): the method is resolved
// on and called directly from staticClass, bypassing the receiver's vtable.
func (b *IRBuilder) BuildStaticMethodCall(object zeus_value.Value, methodName string, args []zeus_value.Value, staticClass *zeus_value.Class, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	b.pushInstr(&Instr{
		Type:   InstrTypeMethodCall,
		Output: result,
		Input:  NewStaticMethodCallInstrInput(object, methodName, args, staticClass),
		Span:   span,
	})
	return result
}

func (b *IRBuilder) BuildGetAccessor(object zeus_value.Value, accessorName string, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	b.pushInstr(&Instr{
		Type:   InstrTypeGetAccessor,
		Output: result,
		Input:  NewGetAccessorInstrInput(object, accessorName),
		Span:   span,
	})
	return result
}

func (b *IRBuilder) BuildSetAccessor(object zeus_value.Value, accessorName string, value zeus_value.Value, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	b.pushInstr(&Instr{
		Type:   InstrTypeSetAccessor,
		Output: result,
		Input:  NewSetAccessorInstrInput(object, accessorName, value),
		Span:   span,
	})
	return result
}

// BuildThrow builds a THROW instruction
func (b *IRBuilder) BuildThrow(classId int, objectPtr zeus_value.Value, sourceFile string, span *token.Span) {
	sourceLine := 0
	if span != nil {
		sourceLine = span.Start.Line
	}
	b.pushInstr(&Instr{
		Type:  InstrTypeThrow,
		Input: NewThrowInstrInput(classId, objectPtr, sourceFile, sourceLine),
		Span:  span,
	})
}

// BuildPushHandler builds a PUSH_HANDLER instruction to register an exception handler
func (b *IRBuilder) BuildPushHandler(handlerBlock *BasicBlock, tryBodyBlock *BasicBlock, classIds []int, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypePushHandler,
		Input: NewPushHandlerInstrInput(handlerBlock, tryBodyBlock, classIds),
		Span:  span,
	})
}

// BuildPopHandler builds a POP_HANDLER instruction to unregister an exception handler
func (b *IRBuilder) BuildPopHandler(span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypePopHandler,
		Span: span,
	})
}

// BuildCheckException builds a CHECK_EXCEPTION instruction that branches based on exception state
func (b *IRBuilder) BuildCheckException(handlerBlock *BasicBlock, continueBlock *BasicBlock, span *token.Span) {
	b.pushInstr(&Instr{
		Type:  InstrTypeCheckException,
		Input: NewCheckExceptionInstrInput(handlerBlock, continueBlock),
		Span:  span,
	})
}

// BuildGetException builds a GET_EXCEPTION instruction to retrieve the current exception
func (b *IRBuilder) BuildGetException(expectedType zeus_value.ValueType, span *token.Span) zeus_value.Value {
	result := b.createTempVariable(span)
	result.ValueType = expectedType
	b.pushInstr(&Instr{
		Type:   InstrTypeGetException,
		Output: result,
		Span:   span,
	})
	return result
}

// BuildClearException builds a CLEAR_EXCEPTION instruction to clear the current exception state
func (b *IRBuilder) BuildClearException(span *token.Span) {
	b.pushInstr(&Instr{
		Type: InstrTypeClearException,
		Span: span,
	})
}

func (b *IRBuilder) GetFunctionBlocks() []*BasicBlock {
	functionBlocks := []*BasicBlock{}

	for _, instr := range b.instrs {
		if IsFunctionDeclInstr(instr.Type) {
			functionBlocks = append(functionBlocks, AsDeclFuncInstrInput(instr.Input).Body)
		}

		if IsClassMethodDeclInstr(instr.Type) {
			functionBlocks = append(functionBlocks, AsDeclClassMethodInstrInput(instr.Input).Body)
		}
	}

	return functionBlocks
}

func (b *IRBuilder) Optimize() {
	b.optimizeBlocks(b.GetFunctionBlocks())
}

func (b *IRBuilder) String() string {
	output := []string{}

	b.Walk(func(instr *Instr) {
		output = append(output, instr.String())
	}, func(block *BasicBlock) {
		output = append(output, fmt.Sprintf("%d:", block.Id))
	})

	return strings.Join(output, "\n")
}

func (b *IRBuilder) Print() {
	fmt.Println(b.String())
}

func (b *IRBuilder) GetInstrs() []*Instr {
	return b.instrs
}
