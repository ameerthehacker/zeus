package lsp

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ameerthehacker/zeus/internal/module"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// importCompletionKind classifies where inside an import statement the cursor sits.
type importCompletionKind int

const (
	importNone importCompletionKind = iota
	importPath                      // inside the `from "..."` path string
	importSymbol                    // inside the `import { ... }` name list
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
		return s.importPathCompletions(docPath, ctx.pathPrefix), true
	case importSymbol:
		return s.importSymbolCompletions(docPath, ctx.moduleStr), true
	default:
		return nil, false
	}
}

// importPathCompletions lists the .zs files and subdirectories that can complete the partial
// import path, resolved relative to the importing document's directory. Files are offered
// without the .zs extension (the form imports use); directories keep a trailing slash.
func (s *Server) importPathCompletions(docPath, pathPrefix string) []protocol.CompletionItem {
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
	selfBase := filepath.Base(docPath)
	lowerName := strings.ToLower(namePart)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(strings.ToLower(name), lowerName) {
			continue
		}
		if e.IsDir() {
			items = append(items, protocol.CompletionItem{
				Label:      name + "/",
				Kind:       protocol.CompletionItemKindFolder,
				InsertText: name + "/",
				Detail:     "directory",
			})
			continue
		}
		if !strings.HasSuffix(name, ".zs") || name == selfBase {
			continue
		}
		base := strings.TrimSuffix(name, ".zs")
		items = append(items, protocol.CompletionItem{
			Label:      base,
			Kind:       protocol.CompletionItemKindFile,
			InsertText: base,
			Detail:     "module",
		})
	}
	return items
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
