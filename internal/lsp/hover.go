package lsp

import (
	"fmt"

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

	line := int(position.Line)
	col := int(position.Character)

	word, isMember := wordAt(docInfo.Content, line, col)
	if word == "" {
		return nil
	}

	// Member access: resolve the receiver chain that precedes this word and describe the
	// member. parseMemberContext is evaluated at the word's start column so the word itself
	// is treated as the (empty-partial) member being hovered.
	if isMember && docInfo.IRModule != nil {
		wordStart := wordStartColumn(docInfo.Content, line, col)
		if mctx, ok := parseMemberContext(docInfo.Content, line, wordStart); ok {
			if r, ok := resolveReceiverChain(docInfo.IRModule, mctx.chain); ok {
				if desc, ok := describeMember(r, word); ok {
					return markdownHover(desc)
				}
			}
		}
		return nil
	}

	// Built-in type keyword.
	if _, ok := token.DataTypes[word]; ok {
		return markdownHover(fmt.Sprintf("```zeus\n%s\n```\n%s", word, dataTypeDescription(word)))
	}
	// Language keyword.
	if _, ok := token.Keywords[word]; ok {
		return markdownHover(fmt.Sprintf("**keyword** `%s` — %s", word, keywordDescription(word)))
	}

	// User-declared symbol (variable, function, or class).
	if docInfo.IRModule != nil {
		if desc, ok := describeSymbol(docInfo.IRModule, word); ok {
			return markdownHover(desc)
		}
	}

	return nil
}

// wordStartColumn returns the rune column at which the identifier under `col` begins.
func wordStartColumn(content string, line, col int) int {
	runes := []rune(lineAt(content, line))
	if col > len(runes) {
		col = len(runes)
	}
	start := col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	return start
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
			m.owner.SourceName(), staticKw, accessModifierString(m.access)+readonly, name, typeString(m.valueType)), true
	case memberAccessor:
		return fmt.Sprintf("```zeus\n(accessor) %s.%s — %s\n```", m.owner.SourceName(), name, m.accessor.String()), true
	default: // memberMethod
		return fmt.Sprintf("```zeus\n(method) %s.%s%s%s\n```",
			m.owner.SourceName(), staticKw, name, funcSignature(m.fn)), true
	}
}

// describeSymbol renders a markdown description of a top-level-visible symbol.
func describeSymbol(irModule *ir.IRModule, name string) (string, bool) {
	sym := symbolByName(irModule, name)
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
	if fn := zeus_value.AsFunction(sym); fn != nil {
		return fmt.Sprintf("```zeus\nfunction %s%s\n```", fn.SourceName(), funcSignature(fn)), true
	}
	if v := zeus_value.AsVar(sym); v != nil {
		return fmt.Sprintf("```zeus\n(variable) %s: %s\n```", name, typeString(v.ValueType)), true
	}
	if o := zeus_value.AsObject(sym); o != nil {
		return fmt.Sprintf("```zeus\n(variable) %s: %s\n```", name, typeString(o.ValueType)), true
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
