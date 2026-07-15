package lsp

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// getHover returns type information for the symbol or member under the cursor. It returns
// nil (no hover) when the cursor is not on a resolvable identifier, which the LSP renders
// as "no hover info" rather than an error.
func (s *Server) getHover(uri protocol.DocumentURI, position protocol.Position) *protocol.Hover {
	docInfo, ok := s.doc(uri)
	if !ok {
		return nil
	}

	// Prefer the AST + semantic model: resolve the identifier under the cursor to its binding.
	if docInfo.AST != nil {
		if id, member := identifierAt(docInfo.AST, zeusPos(position)); id != nil {
			if member != nil {
				// Member access: resolve the receiver and describe the member under the cursor.
				if r, ok := resolveReceiver(docInfo, member.Object); ok {
					if desc, ok := describeMember(r, id.Name.Value); ok {
						return markdownHover(desc)
					}
				}
				return nil
			}
			// Plain reference: describe the bound symbol (scope-correct), falling back to a name
			// lookup when the identifier has no recorded binding.
			if sym, ok := docInfo.Semantic.SymbolAt(id); ok {
				if desc, ok := renderSymbol(sym, id.Name.Value); ok {
					return markdownHover(desc)
				}
			}
			if docInfo.IRModule != nil {
				if desc, ok := describeSymbol(docInfo.IRModule, id.Name.Value); ok {
					return markdownHover(desc)
				}
			}
			return nil
		}
	}

	// Not on a parsed identifier (a keyword, a built-in type name in an annotation, or a syntax
	// hole): fall back to a lexeme scan and describe keywords/types/symbols by name.
	word, _ := wordAt(docInfo.Content, int(position.Line), int(position.Character))
	if word == "" {
		return nil
	}
	if _, ok := token.DataTypes[word]; ok {
		return markdownHover(fmt.Sprintf("```zeus\n%s\n```\n%s", word, dataTypeDescription(word)))
	}
	if _, ok := token.Keywords[word]; ok {
		return markdownHover(fmt.Sprintf("**keyword** `%s` — %s", word, keywordDescription(word)))
	}
	if docInfo.IRModule != nil {
		if desc, ok := describeSymbol(docInfo.IRModule, word); ok {
			return markdownHover(desc)
		}
	}
	return nil
}

// describeMember renders a markdown description of a named member on a receiver.
func describeMember(r receiver, name string) (string, bool) {
	m, ok := lookupMember(r, name)
	if !ok {
		return "", false
	}
	staticKw := ""
	if m.isStatic {
		staticKw = "static "
	}
	switch m.kind {
	case memberProperty:
		readonly := ""
		if m.isReadonly {
			readonly = "readonly "
		}
		return fmt.Sprintf("```zeus\n(property) %s.%s%s%s: %s\n```",
			m.ownerName, staticKw, accessModifierString(m.access)+readonly, name, typeString(m.valueType)), true
	case memberAccessor:
		return fmt.Sprintf("```zeus\n(accessor) %s.%s — %s\n```", m.ownerName, name, m.accessor.String()), true
	default: // memberMethod
		return fmt.Sprintf("```zeus\n(method) %s.%s%s%s\n```",
			m.ownerName, staticKw, name, funcSignature(m.fn)), true
	}
}

// describeSymbol renders a markdown description of a top-level-visible symbol, looked up by name.
func describeSymbol(irModule *ir.IRModule, name string) (string, bool) {
	return renderSymbol(symbolByName(irModule, name), name)
}

// renderSymbol renders a markdown description of an already-resolved symbol value. Sharing this
// between name-based lookup and the binding index means hover text is identical whichever path
// resolved the symbol.
func renderSymbol(sym zeus_value.Value, name string) (string, bool) {
	if sym == nil {
		return "", false
	}

	if class := zeus_value.AsClass(sym); class != nil {
		header := "class " + class.SourceName()
		if class.ParentClass != nil {
			header += " extends " + class.ParentClass.SourceName()
		}
		return fmt.Sprintf("```zeus\n%s\n```", header), true
	}
	if iface := zeus_value.AsInterface(sym); iface != nil {
		header := "interface " + iface.Name
		if len(iface.Parents) > 0 {
			parents := make([]string, 0, len(iface.Parents))
			for _, p := range iface.Parents {
				parents = append(parents, p.Name)
			}
			header += " extends " + strings.Join(parents, ", ")
		}
		return fmt.Sprintf("```zeus\n%s\n```", header), true
	}
	if fn := zeus_value.AsFunction(sym); fn != nil {
		return fmt.Sprintf("```zeus\nfunction %s%s\n```", fn.SourceName(), funcSignature(fn)), true
	}
	if v := zeus_value.AsVar(sym); v != nil {
		return fmt.Sprintf("```zeus\n(variable) %s: %s\n```", name, typeString(v.ValueType)), true
	}
	if o := zeus_value.AsObject(sym); o != nil {
		return fmt.Sprintf("```zeus\n(variable) %s: %s\n```", name, typeString(o.ValueType)), true
	}
	if rc := zeus_value.AsRefCellVar(sym); rc != nil {
		return fmt.Sprintf("```zeus\n(variable) %s: %s\n```", name, typeString(rc.ValueType)), true
	}
	return "", false
}

// markdownHover wraps markdown content in an LSP Hover response.
func markdownHover(value string) *protocol.Hover {
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: value,
		},
	}
}
