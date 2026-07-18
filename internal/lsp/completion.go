package lsp

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// getCompletions returns completion items for the document at the given position.
// If the cursor sits at a member access (`receiver.`), it offers the receiver's members;
// otherwise it offers keywords, built-in types, and in-scope symbols.
func (s *Server) getCompletions(uri protocol.DocumentURI, position protocol.Position) *protocol.CompletionList {
	docInfo, _ := s.doc(uri)

	line := int(position.Line)
	col := int(position.Character)

	content := ""
	if docInfo != nil {
		content = docInfo.Content
	}
	empty := &protocol.CompletionList{IsIncomplete: false, Items: []protocol.CompletionItem{}}

	// Import completion: `from "./|"` (paths) or `import { | } from "./mod"` (exported symbols).
	// Checked first because an import statement is unambiguous and must not fall through to
	// keyword/member completion — and the `from "..."` path is itself a string literal, so it must
	// be handled before the string/comment guard below.
	if items, ok := s.importCompletions(uri.Filename(), content, position); ok {
		return &protocol.CompletionList{IsIncomplete: false, Items: items}
	}

	// Suppress completion inside ordinary string literals and comments: offering variables,
	// keywords, or members there is noise. Import paths are already handled above, and template
	// `${ ... }` interpolations are treated as code (not suppressed).
	if inStringOrComment(content, line, col) {
		return empty
	}

	// Member completion: `obj.` or `obj.parti|`. This is detected from the text alone, so a '.'
	// never falls back to keyword/symbol completions (which would be noise) — even when the
	// document failed to produce an IR module. Members are only offered when the receiver's
	// type actually resolves; otherwise the result is empty.
	if mctx, ok := parseMemberContext(content, line, col); ok {
		if docInfo != nil && docInfo.IRModule != nil {
			if r, ok := resolveReceiverChain(docInfo.IRModule, mctx.chain); ok {
				return &protocol.CompletionList{IsIncomplete: false, Items: memberCompletionItems(r, mctx.partial)}
			}
		}
		return empty
	}

	items := keywordAndTypeCompletionItems()
	seen := map[string]bool{}
	// Keywords and built-in type names are already listed; record them so a primordial class of the
	// same name (e.g. the `string` class vs the `string` type) is not offered twice.
	for _, item := range items {
		seen[item.Label] = true
	}
	if docInfo != nil && docInfo.IRModule != nil {
		for _, item := range symbolCompletionItems(docInfo.IRModule, position) {
			seen[item.Label] = true
			items = append(items, item)
		}
	}
	// Globals that are visible without import — the `console` singleton and user `global`s, plus
	// primordial namespaces/types (`Math`, `Error`, ...) and free functions (`setTimeout`, ...) —
	// only enter a module's symbol table lazily on first reference, so they are added explicitly
	// here; otherwise they never appear while typing.
	items = append(items, globalCompletionItems(seen)...)

	return &protocol.CompletionList{IsIncomplete: false, Items: items}
}

// globalCompletionItems returns completion items for every symbol available without import:
// ambient globals (`console` + user `global`s), primordial classes with identifier names
// (`Math`, `Error`, `Console`, `string`), and primordial free functions (`setTimeout`, ...).
// Names already emitted (via `seen`), compiler-internal names, and non-identifier primordials
// (e.g. array classes like `u8[]`) are skipped.
func globalCompletionItems(seen map[string]bool) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	add := func(name string, item protocol.CompletionItem) {
		if seen[name] || isInternalSymbolName(name) || !isIdentifierName(name) {
			return
		}
		seen[name] = true
		items = append(items, item)
	}

	for _, info := range zeus_value.AllAmbientGlobals() {
		detail := "global"
		if info.ValueType != nil {
			detail = info.ValueType.String()
		}
		add(info.Name, protocol.CompletionItem{
			Label:         info.Name,
			Kind:          protocol.CompletionItemKindVariable,
			Detail:        detail,
			Documentation: "Global " + info.Name,
		})
	}
	for _, class := range zeus_value.Registry.GetAllClasses() {
		name := class.SourceName()
		add(name, protocol.CompletionItem{
			Label:         name,
			Kind:          protocol.CompletionItemKindClass,
			Detail:        "class " + name,
			Documentation: "Built-in class " + name,
		})
	}
	for _, fn := range zeus_value.Registry.GetAllFunctions() {
		name := fn.SourceName()
		add(name, protocol.CompletionItem{
			Label:            name,
			Kind:             protocol.CompletionItemKindFunction,
			Detail:           funcSignature(fn),
			Documentation:    "Built-in function " + name + funcSignature(fn),
			InsertText:       name + "($0)",
			InsertTextFormat: protocol.InsertTextFormatSnippet,
		})
	}
	return items
}

// memberCompletionItems lists the members (properties, accessors, methods) offered on a
// receiver, honouring the static/instance context and hiding compiler-internal members.
// Derived members shadow same-named base members. `partial` is advisory — the editor does
// the final filtering — so all members are returned.
//
// Only public members are offered. Member access at a completion site is almost always
// external (`obj.`), where private/protected members are inaccessible; offering them would
// suggest code that fails to compile and surface compiler internals (e.g. a string's private
// `data` buffer).
func memberCompletionItems(r receiver, partial string) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	eachMember(r, func(m member) {
		if !m.isPublic() {
			return
		}
		items = append(items, memberCompletionItem(m))
	})
	return items
}

// memberCompletionItem renders a resolved member as a completion item.
func memberCompletionItem(m member) protocol.CompletionItem {
	switch m.kind {
	case memberProperty:
		detail := typeString(m.valueType)
		if m.isReadonly {
			detail = "readonly " + detail
		}
		return protocol.CompletionItem{
			Label:         m.name,
			Kind:          protocol.CompletionItemKindField,
			Detail:        accessModifierString(m.access) + detail,
			Documentation: fmt.Sprintf("%s.%s: %s", m.ownerName, m.name, detail),
		}
	case memberAccessor:
		kinds := []string{}
		if m.accessor.Getter != nil {
			kinds = append(kinds, "get")
		}
		if m.accessor.Setter != nil {
			kinds = append(kinds, "set")
		}
		return protocol.CompletionItem{
			Label:         m.name,
			Kind:          protocol.CompletionItemKindProperty,
			Detail:        fmt.Sprintf("%s(%s) %s", accessModifierString(m.access), strings.Join(kinds, "/"), typeString(m.valueType)),
			Documentation: fmt.Sprintf("accessor %s.%s: %s", m.ownerName, m.name, typeString(m.valueType)),
		}
	default: // memberMethod
		sig := funcSignature(m.fn)
		return protocol.CompletionItem{
			Label:            m.name,
			Kind:             protocol.CompletionItemKindMethod,
			Detail:           accessModifierString(m.access) + sig,
			Documentation:    fmt.Sprintf("%s.%s%s", m.ownerName, m.name, sig),
			InsertText:       m.name + "($0)",
			InsertTextFormat: protocol.InsertTextFormatSnippet,
		}
	}
}

// keywordAndTypeCompletionItems returns completion items for all Zeus keywords and built-in
// data types.
func keywordAndTypeCompletionItems() []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	for keyword := range token.Keywords {
		items = append(items, protocol.CompletionItem{
			Label:  keyword,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: keywordDescription(keyword),
		})
	}
	for dataType := range token.DataTypes {
		items = append(items, protocol.CompletionItem{
			Label:  dataType,
			Kind:   protocol.CompletionItemKindTypeParameter,
			Detail: dataTypeDescription(dataType),
		})
	}
	return items
}

// symbolCompletionItems returns completion items for user-declared symbols (variables,
// functions, classes) that are declared at or before the cursor position.
func symbolCompletionItems(irModule *ir.IRModule, cursorPosition protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	seen := make(map[string]bool)

	cursorLine := int(cursorPosition.Line) + 1
	cursorColumn := int(cursorPosition.Character) + 1

	for name, value := range irModule.GetAllSymbols() {
		if seen[name] {
			continue
		}
		// Compiler-internal symbols (temp vars, module-init/module-scoped `$` names, `#` accessor
		// helpers) are never user-typable, so they must not surface in completion.
		if isInternalSymbolName(name) {
			continue
		}
		if asVar := zeus_value.AsVar(value); asVar != nil && asVar.IsTempVariable() {
			continue
		}

		var symbolSpan *token.Span
		if asVar := zeus_value.AsVar(value); asVar != nil {
			symbolSpan = asVar.Span
		} else if asFunc := zeus_value.AsFunction(value); asFunc != nil {
			symbolSpan = asFunc.Span
		} else if asClass := zeus_value.AsClass(value); asClass != nil {
			symbolSpan = asClass.Span
		}

		// Skip symbols first declared after the cursor.
		if symbolSpan != nil {
			if symbolSpan.Start.Line > cursorLine {
				continue
			}
			if symbolSpan.Start.Line == cursorLine && symbolSpan.Start.Column > cursorColumn {
				continue
			}
		}

		seen[name] = true

		var kind protocol.CompletionItemKind
		var detail, documentation string

		if asVar := zeus_value.AsVar(value); asVar != nil {
			kind = protocol.CompletionItemKindVariable
			if asVar.ValueType != nil {
				detail = asVar.ValueType.String()
				documentation = fmt.Sprintf("Variable of type %s", asVar.ValueType.String())
			} else {
				detail = "variable"
			}
		} else if asFunc := zeus_value.AsFunction(value); asFunc != nil {
			kind = protocol.CompletionItemKindFunction
			detail = funcSignature(asFunc)
			documentation = "Function " + detail
		} else if asClass := zeus_value.AsClass(value); asClass != nil {
			kind = protocol.CompletionItemKindClass
			detail = "class " + asClass.SourceName()
			documentation = detail
		} else {
			continue
		}

		items = append(items, protocol.CompletionItem{
			Label:         name,
			Kind:          kind,
			Detail:        detail,
			Documentation: documentation,
		})
	}

	return items
}

// keywordDescription returns a human-readable description for a keyword.
func keywordDescription(keyword string) string {
	descriptions := map[string]string{
		"let":        "Variable declaration",
		"const":      "Constant declaration",
		"function":   "Function declaration",
		"return":     "Return statement",
		"if":         "If statement",
		"else":       "Else statement",
		"while":      "While loop",
		"for":        "For loop",
		"true":       "Boolean true",
		"false":      "Boolean false",
		"import":     "Import statement",
		"export":     "Export statement",
		"from":       "Import from",
		"class":      "Class declaration",
		"extends":    "Class inheritance",
		"private":    "Private access modifier",
		"public":     "Public access modifier",
		"protected":  "Protected access modifier",
		"new":        "Object instantiation",
		"null":       "Null value",
		"try":        "Try block",
		"catch":      "Catch block",
		"throw":      "Throw statement",
		"break":      "Break statement",
		"continue":   "Continue statement",
		"interface":  "Interface declaration",
		"implements": "Interface implementation",
		"as":         "Type cast (e.g. `x as i32`)",
	}
	if desc, ok := descriptions[keyword]; ok {
		return desc
	}
	return "Keyword: " + keyword
}

// dataTypeDescription returns a human-readable description for a built-in data type.
func dataTypeDescription(typeName string) string {
	descriptions := map[string]string{
		"void":    "Void type",
		"i8":      "8-bit signed integer",
		"i16":     "16-bit signed integer",
		"i32":     "32-bit signed integer",
		"i64":     "64-bit signed integer",
		"u8":      "8-bit unsigned integer",
		"u16":     "16-bit unsigned integer",
		"u32":     "32-bit unsigned integer",
		"u64":     "64-bit unsigned integer",
		"f32":     "32-bit floating point",
		"f64":     "64-bit floating point",
		"boolean": "Boolean type",
		"bool":    "Boolean type",
		"string":  "String type",
		"null":    "Null type",
	}
	if desc, ok := descriptions[typeName]; ok {
		return desc
	}
	return "Type: " + typeName
}
