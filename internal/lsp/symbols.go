package lsp

import (
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// spanToRange converts a Zeus span (1-based, inclusive end) to an LSP range (0-based,
// exclusive end), mirroring the diagnostic conversion. A nil span maps to the origin.
func spanToRange(span *token.Span) protocol.Range {
	if span == nil {
		return protocol.Range{}
	}
	return protocol.Range{
		Start: protocol.Position{
			Line:      uint32(max0(span.Start.Line - 1)),
			Character: uint32(max0(span.Start.Column - 1)),
		},
		End: protocol.Position{
			Line:      uint32(max0(span.End.Line - 1)),
			Character: uint32(max0(span.End.Column)),
		},
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// getDefinition resolves the declaration location of the symbol or member under the cursor.
// It returns an empty slice when nothing resolves (a valid "no definition" response).
func (s *Server) getDefinition(uri protocol.DocumentURI, position protocol.Position) []protocol.Location {
	none := []protocol.Location{}
	docInfo, ok := s.doc(uri)
	if !ok || docInfo.IRModule == nil {
		return none
	}

	// Prefer the AST + semantic model: resolve the identifier under the cursor to a declaration.
	if docInfo.AST != nil {
		if id, member := identifierAt(docInfo.AST, zeusPos(position)); id != nil {
			var span *token.Span
			if member != nil {
				if r, ok := resolveReceiver(docInfo, member.Object); ok {
					span = memberSpan(r, id.Name.Value)
				}
			} else if sym, ok := docInfo.Semantic.SymbolAt(id); ok {
				// Scope-correct: the recorded binding is the symbol actually in scope here.
				span = symbolSpan(sym)
			} else {
				span = symbolSpan(symbolByName(docInfo.IRModule, id.Name.Value))
			}
			if span == nil {
				return none
			}
			return []protocol.Location{{URI: uri, Range: spanToRange(span)}}
		}
	}

	// Not on a parsed identifier (e.g. a type name in an annotation): resolve by name.
	word, _ := wordAt(docInfo.Content, int(position.Line), int(position.Character))
	if word == "" {
		return none
	}
	span := symbolSpan(symbolByName(docInfo.IRModule, word))
	if span == nil {
		return none
	}
	return []protocol.Location{{URI: uri, Range: spanToRange(span)}}
}

// symbolSpan returns the declaration span of a symbol value, or nil.
func symbolSpan(value zeus_value.Value) *token.Span {
	if value == nil {
		return nil
	}
	if v := zeus_value.AsVar(value); v != nil {
		return v.Span
	}
	if fn := zeus_value.AsFunction(value); fn != nil {
		return fn.Span
	}
	if class := zeus_value.AsClass(value); class != nil {
		return class.Span
	}
	if o := zeus_value.AsObject(value); o != nil {
		return o.Span
	}
	// An escaped local (captured by a closure) is stored as a ref cell; its declaration span still
	// points at the original `let`/`const`.
	if rc := zeus_value.AsRefCellVar(value); rc != nil {
		return rc.Span
	}
	return nil
}

// memberSpan returns the declaration span of a named member on a receiver, or nil.
func memberSpan(r receiver, name string) *token.Span {
	if m, ok := lookupMember(r, name); ok {
		return m.span
	}
	return nil
}

// getDocumentSymbols returns an outline of the user-defined classes and functions in the
// document. Primordial and synthesized symbols (Console, array classes, etc.) are excluded.
func (s *Server) getDocumentSymbols(uri protocol.DocumentURI) []protocol.DocumentSymbol {
	symbols := []protocol.DocumentSymbol{}
	docInfo, ok := s.doc(uri)
	if !ok || docInfo.IRModule == nil {
		return symbols
	}

	seen := map[string]bool{}
	for _, value := range docInfo.IRModule.GetAllSymbols() {
		if class := zeus_value.AsClass(value); class != nil {
			// Only user-defined classes have a source span and no primordial name.
			if class.PrimordialName != "" || class.Span == nil || seen["class:"+class.SourceName()] {
				continue
			}
			seen["class:"+class.SourceName()] = true
			symbols = append(symbols, classDocumentSymbol(class))
			continue
		}
		if fn := zeus_value.AsFunction(value); fn != nil {
			// User functions carry their source name in OriginalName; primordials do not.
			if fn.OriginalName == "" || fn.Span == nil || fn.Class != nil || seen["fn:"+fn.OriginalName] {
				continue
			}
			seen["fn:"+fn.OriginalName] = true
			symbols = append(symbols, protocol.DocumentSymbol{
				Name:           fn.OriginalName,
				Detail:         funcSignature(fn),
				Kind:           protocol.SymbolKindFunction,
				Range:          spanToRange(fn.Span),
				SelectionRange: spanToRange(fn.Span),
			})
		}
	}

	return symbols
}

// classDocumentSymbol builds a document symbol for a user class, nesting its own (non-inherited)
// properties, accessors, and methods as children.
func classDocumentSymbol(class *zeus_value.Class) protocol.DocumentSymbol {
	children := []protocol.DocumentSymbol{}

	for _, prop := range class.Properties {
		children = append(children, protocol.DocumentSymbol{
			Name:           prop.Property.Name,
			Detail:         typeString(prop.Property.ValueType),
			Kind:           protocol.SymbolKindField,
			Range:          spanToRange(prop.Property.Span),
			SelectionRange: spanToRange(prop.Property.Span),
		})
	}
	for _, acc := range class.Accessors {
		span := (*token.Span)(nil)
		if acc.Getter != nil {
			span = acc.Getter.Span
		} else if acc.Setter != nil {
			span = acc.Setter.Span
		}
		children = append(children, protocol.DocumentSymbol{
			Name:           acc.Name,
			Detail:         acc.String(),
			Kind:           protocol.SymbolKindProperty,
			Range:          spanToRange(span),
			SelectionRange: spanToRange(span),
		})
	}
	for _, m := range class.Methods {
		name := m.Method.SourceName()
		if m.IsAccessor {
			continue
		}
		kind := protocol.SymbolKindMethod
		if name == token.CONSTRUCTOR_METHOD_NAME {
			kind = protocol.SymbolKindConstructor
		}
		children = append(children, protocol.DocumentSymbol{
			Name:           name,
			Detail:         funcSignature(m.Method),
			Kind:           kind,
			Range:          spanToRange(m.Method.Span),
			SelectionRange: spanToRange(m.Method.Span),
		})
	}

	return protocol.DocumentSymbol{
		Name:           class.SourceName(),
		Kind:           protocol.SymbolKindClass,
		Range:          spanToRange(class.Span),
		SelectionRange: spanToRange(class.Span),
		Children:       children,
	}
}
