package lsp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// importCompletionKind classifies where inside an import statement the cursor sits.
type importCompletionKind int

const (
	importNone   importCompletionKind = iota
	importPath                        // inside the `from "..."` path string
	importSymbol                      // inside the `import { ... }` name list
)

type importCompletionContext struct {
	kind       importCompletionKind
	pathPrefix string // importPath: the partial path already typed inside the quotes
	moduleStr  string // importSymbol: the module string from the `from "..."` clause (may be "")
}

// startsWithKeyword reports whether s begins with the given keyword as a whole word.
func startsWithKeyword(s, keyword string) bool {
	if !strings.HasPrefix(s, keyword) {
		return false
	}
	rest := s[len(keyword):]
	return rest == "" || !isIdentByte(rest[0])
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// detectImportContext determines whether the cursor is completing an import path or an imported
// symbol on the given (single) line. It is heuristic and handles single-line import statements,
// which is how imports are normally written.
func detectImportContext(line string, col int) importCompletionContext {
	trimmed := strings.TrimLeft(line, " \t")
	if !startsWithKeyword(trimmed, "import") {
		return importCompletionContext{kind: importNone}
	}
	prefix := runePrefix(line, col)

	// An odd number of quotes before the cursor means the cursor is inside the path string
	// (an import has at most one string — the module path).
	if strings.Count(prefix, `"`)%2 == 1 {
		lastQuote := strings.LastIndex(prefix, `"`)
		return importCompletionContext{kind: importPath, pathPrefix: prefix[lastQuote+1:]}
	}

	// A `{` before the cursor with no matching `}` yet means we are inside the name list.
	if openBrace := strings.LastIndex(prefix, "{"); openBrace != -1 && openBrace > strings.LastIndex(prefix, "}") {
		return importCompletionContext{kind: importSymbol, moduleStr: moduleStringOf(line)}
	}
	return importCompletionContext{kind: importNone}
}

// moduleStringOf extracts the module path from the `from "..."` clause of an import line.
func moduleStringOf(line string) string {
	idx := strings.Index(line, "from")
	if idx == -1 {
		return ""
	}
	rest := line[idx+len("from"):]
	open := strings.Index(rest, `"`)
	if open == -1 {
		return ""
	}
	close := strings.Index(rest[open+1:], `"`)
	if close == -1 {
		return ""
	}
	return rest[open+1 : open+1+close]
}

// importCompletions returns import path or symbol completions when the cursor is inside an
// import statement, or ok=false otherwise. docPath is the importing document's file path.
func (s *Server) importCompletions(docPath, content string, position protocol.Position) ([]protocol.CompletionItem, bool) {
	line := lineAt(content, int(position.Line))
	ctx := detectImportContext(line, int(position.Character))
	switch ctx.kind {
	case importPath:
		return s.importPathCompletions(docPath, ctx.pathPrefix, position), true
	case importSymbol:
		return s.importSymbolCompletions(docPath, ctx.moduleStr), true
	default:
		return nil, false
	}
}

// importPathCompletions lists the .zs files and subdirectories that can complete the partial
// import path, resolved relative to the importing document's directory. Files are offered
// without the .zs extension (the form imports use); directories keep a trailing slash. A bare
// sibling (no `./`, `../`, or `/` already typed) is inserted with an explicit `./` prefix, so
// selecting it yields a valid relative specifier like `./relative` rather than `relative`.
func (s *Server) importPathCompletions(docPath, pathPrefix string, position protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	// Standard-library ("@...") paths resolve outside the workspace; not offered here.
	if docPath == "" || strings.HasPrefix(pathPrefix, "@") {
		return items
	}

	// Split the typed prefix into a directory part (already committed) and the name being typed.
	dirPart, namePart := "", pathPrefix
	if slash := strings.LastIndex(pathPrefix, "/"); slash != -1 {
		dirPart = pathPrefix[:slash+1]
		namePart = pathPrefix[slash+1:]
	}
	baseDir := filepath.Join(filepath.Dir(docPath), dirPart)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return items
	}
	// Replace the whole typed path prefix (inside the quotes) so the inserted specifier is exact,
	// independent of the client's word-boundary heuristics around `.` and `/`.
	replaceRange := protocol.Range{
		Start: protocol.Position{Line: position.Line, Character: uint32(max0(int(position.Character) - len([]rune(pathPrefix))))},
		End:   position,
	}
	selfBase := filepath.Base(docPath)
	lowerName := strings.ToLower(namePart)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(strings.ToLower(name), lowerName) {
			continue
		}
		if e.IsDir() {
			items = append(items, importPathItem(name+"/", dirPart+name+"/", protocol.CompletionItemKindFolder, "directory", replaceRange))
			continue
		}
		if !strings.HasSuffix(name, ".zs") || name == selfBase {
			continue
		}
		base := strings.TrimSuffix(name, ".zs")
		items = append(items, importPathItem(base, dirPart+base, protocol.CompletionItemKindFile, "module", replaceRange))
	}
	return items
}

// importPathItem builds an import-path completion item. label is what the editor lists; specifier
// is the full path relative to the opening quote, canonicalized to an explicit relative form
// (`./x`) when it is a bare sibling. The specifier is emitted via a TextEdit that replaces the
// whole typed prefix so insertion is deterministic.
func importPathItem(label, specifier string, kind protocol.CompletionItemKind, detail string, replaceRange protocol.Range) protocol.CompletionItem {
	if !strings.HasPrefix(specifier, ".") && !strings.HasPrefix(specifier, "/") {
		specifier = "./" + specifier
	}
	return protocol.CompletionItem{
		Label:      label,
		Kind:       kind,
		Detail:     detail,
		InsertText: specifier,
		TextEdit:   &protocol.TextEdit{Range: replaceRange, NewText: specifier},
	}
}

// importSymbolCompletions lists the names a module exports, so `import { | } from "./mod"`
// completes to that module's exports. The module is resolved and parsed on demand.
func (s *Server) importSymbolCompletions(docPath, moduleStr string) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	if docPath == "" || moduleStr == "" {
		return items
	}
	absPath := module.ResolveFilePath(docPath, moduleStr)
	irModule := s.makeModuleResolver(docPath)(absPath)
	if irModule == nil {
		return items
	}
	for name, value := range irModule.GetExportedSymbols() {
		kind := protocol.CompletionItemKindValue
		detail := "export"
		if fn := zeus_value.AsFunction(value); fn != nil {
			kind = protocol.CompletionItemKindFunction
			detail = "function " + funcSignature(fn)
		} else if class := zeus_value.AsClass(value); class != nil {
			kind = protocol.CompletionItemKindClass
			detail = "class " + class.SourceName()
		}
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   kind,
			Detail: detail,
		})
	}
	return items
}

// importPathDefinition resolves the module a `from "..."` clause points to when the cursor sits on
// that path string, returning a Location at the top of the resolved file. It handles both relative
// (`./x`) and standard-library (`@std/x`) specifiers via module.ResolveFilePath.
func (s *Server) importPathDefinition(docInfo *DocumentInfo, docPath string, pos token.Position) (protocol.Location, bool) {
	if docInfo == nil || docInfo.AST == nil || docPath == "" {
		return protocol.Location{}, false
	}
	for _, stmt := range docInfo.AST.Statements {
		imp, ok := stmt.(*ast.ImportStmtNode)
		if !ok || imp.Source == nil || !spanContains(imp.Source.Span, pos) {
			continue
		}
		absPath := module.ResolveFilePath(docPath, imp.Source.Value)
		if _, err := os.Stat(absPath); err != nil {
			return protocol.Location{}, false
		}
		return protocol.Location{URI: uri.File(absPath)}, true
	}
	return protocol.Location{}, false
}

// importedSymbolLocation resolves an imported symbol name to its declaration in the module it was
// imported from, returning a Location whose URI is that source file (not the importing document).
//
// `resolved` is the scope-correct binding under the cursor (or nil when none was found). When it is
// non-nil and its declaration span differs from the export's, the cursor's name is shadowed by a
// local or parameter, so we decline (ok=false) and let same-file resolution handle it — otherwise a
// local `add` that shadows an imported `add` would wrongly navigate to the import.
//
// It also returns ok=false when name is not imported, the module cannot be resolved, or the export
// has no declaration span.
func (s *Server) importedSymbolLocation(docInfo *DocumentInfo, docPath, name string, resolved zeus_value.Value) (protocol.Location, bool) {
	if docInfo == nil || docInfo.AST == nil || docPath == "" {
		return protocol.Location{}, false
	}
	for _, stmt := range docInfo.AST.Statements {
		imp, ok := stmt.(*ast.ImportStmtNode)
		if !ok || imp.Source == nil || !importsName(imp, name) {
			continue
		}
		absPath := module.ResolveFilePath(docPath, imp.Source.Value)
		m := s.makeModuleResolver(docPath)(absPath)
		if m == nil {
			return protocol.Location{}, false
		}
		sym, ok := m.GetExportedSymbol(name)
		if !ok {
			return protocol.Location{}, false
		}
		span := symbolSpan(sym)
		if span == nil {
			return protocol.Location{}, false
		}
		// A local/parameter shadowing this import resolves to a different declaration; don't hijack it.
		if resolved != nil && !sameSpan(symbolSpan(resolved), span) {
			return protocol.Location{}, false
		}
		return protocol.Location{URI: uri.File(absPath), Range: spanToRange(span)}, true
	}
	return protocol.Location{}, false
}

// sameSpan reports whether two spans denote the same source position. Nil spans are equal only to
// nil. Compared by value so it survives a module being regenerated (a fresh symbol with the same
// declaration position still matches).
func sameSpan(a, b *token.Span) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// importsName reports whether an import statement pulls in the given symbol name.
func importsName(imp *ast.ImportStmtNode, name string) bool {
	for _, id := range imp.Imports {
		if id != nil && id.Name != nil && id.Name.Value == name {
			return true
		}
	}
	return false
}

// spanContains reports whether a 1-based source position falls within span (inclusive of both
// ends). A nil span contains nothing.
func spanContains(span *token.Span, pos token.Position) bool {
	if span == nil {
		return false
	}
	if pos.Line < span.Start.Line || (pos.Line == span.Start.Line && pos.Column < span.Start.Column) {
		return false
	}
	if pos.Line > span.End.Line || (pos.Line == span.End.Line && pos.Column > span.End.Column) {
		return false
	}
	return true
}
