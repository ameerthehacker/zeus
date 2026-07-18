package lsp

import (
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// This file contains the position/text analysis and symbol-resolution helpers shared
// by completion and hover. Resolution is deliberately text-driven (it inspects the line
// around the cursor) and then answers questions against the IR symbol table, which — after
// parseDocument runs the type checker — holds every declared symbol with a resolved type
// (Walk visits all scopes, so locals are included, not just globals).

// isIdentRune reports whether r can appear in a Zeus identifier. It delegates to the lexer so the
// language server's notion of an identifier stays byte-for-byte consistent with the compiler's
// (including Unicode letters/digits).
func isIdentRune(r rune) bool {
	return lexer.IsIdentifierRune(r)
}

// isInternalSymbolName reports whether a symbol name is compiler-internal and must never be
// offered to the user. Temp IR variables are `%`-prefixed; module-init and module-scoped symbols
// are `$`-prefixed (see internal/module: GetModulePrefix / ModuleInitFuncPrefix); synthesized
// accessor helpers are `#`-prefixed. None are user-typable.
func isInternalSymbolName(name string) bool {
	if name == "" {
		return false
	}
	switch name[0] {
	case '%', '$', '#':
		return true
	}
	return false
}

// isIdentifierName reports whether name is a plain Zeus identifier a user could type. It rejects
// synthesized primordial names that are not identifiers, e.g. array classes like `u8[]`.
func isIdentifierName(name string) bool {
	for i, r := range name {
		if i == 0 {
			if !(r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
			continue
		}
		if !isIdentRune(r) {
			return false
		}
	}
	return name != ""
}

// lineAt returns the given 0-based line of content, or "" if out of range.
func lineAt(content string, line int) string {
	if line < 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if line >= len(lines) {
		return ""
	}
	return strings.TrimSuffix(lines[line], "\r")
}

// runePrefix returns the first `col` runes of s (clamped), so column math is done in runes
// rather than bytes. This keeps multi-byte characters from corrupting the slice.
func runePrefix(s string, col int) string {
	runes := []rune(s)
	if col < 0 {
		col = 0
	}
	if col > len(runes) {
		col = len(runes)
	}
	return string(runes[:col])
}

// docPrefix returns the document text from the start of the file up to (but not including) the
// cursor at 0-based line/character, with column math done in runes to match runePrefix.
func docPrefix(content string, line, character int) string {
	lines := strings.Split(content, "\n")
	if line < 0 {
		line = 0
	}
	if line >= len(lines) {
		line = len(lines) - 1
	}
	var b strings.Builder
	for i := 0; i < line; i++ {
		b.WriteString(strings.TrimSuffix(lines[i], "\r"))
		b.WriteByte('\n')
	}
	b.WriteString(runePrefix(strings.TrimSuffix(lines[line], "\r"), character))
	return b.String()
}

// inStringOrComment reports whether the cursor at 0-based line/character sits inside a string
// literal (`"..."`, `'...'`, or a “ `...` “ template) or a comment (`//` or `/* */`). It runs a
// small state machine over the document prefix, mirroring the lexer's literal/comment forms
// (escape sequences included). Inside a template, `${ ... }` interpolation is treated as code, so
// completion still fires there — only the literal text portions suppress completion.
func inStringOrComment(content string, line, character int) bool {
	runes := []rune(docPrefix(content, line, character))

	const (
		code = iota
		lineComment
		blockComment
		dqString // "..."
		sqString // '...'
		template // `...`
	)
	state := code
	// braceStack tracks nested `${ ... }` interpolations: each entry is the unmatched-brace depth
	// of one interpolation. When it returns to zero the matching `}` restores the template state.
	var braceStack []int

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		switch state {
		case code:
			switch {
			case c == '/' && next == '/':
				state = lineComment
				i++
			case c == '/' && next == '*':
				state = blockComment
				i++
			case c == '"':
				state = dqString
			case c == '\'':
				state = sqString
			case c == '`':
				state = template
			case c == '{' && len(braceStack) > 0:
				braceStack[len(braceStack)-1]++
			case c == '}' && len(braceStack) > 0:
				braceStack[len(braceStack)-1]--
				if braceStack[len(braceStack)-1] == 0 {
					braceStack = braceStack[:len(braceStack)-1]
					state = template
				}
			}
		case lineComment:
			if c == '\n' {
				state = code
			}
		case blockComment:
			if c == '*' && next == '/' {
				state = code
				i++
			}
		case dqString:
			if c == '\\' {
				i++ // skip the escaped rune
			} else if c == '"' {
				state = code
			}
		case sqString:
			if c == '\\' {
				i++
			} else if c == '\'' {
				state = code
			}
		case template:
			if c == '\\' {
				i++
			} else if c == '`' {
				state = code
			} else if c == '$' && next == '{' {
				// Enter interpolation: code context whose matching `}` returns to the template.
				braceStack = append(braceStack, 1)
				state = code
				i++ // consume '{'
			}
		}
	}

	// The cursor is "in string/comment" unless the scan ends in code state — which includes being
	// inside a template interpolation (state == code with a non-empty brace stack).
	return state != code
}

// wordAt returns the identifier surrounding rune-column `col` on `line`, along with whether
// the character immediately before the word is a '.', i.e. the word is a member access.
func wordAt(content string, line, col int) (word string, isMember bool) {
	text := lineAt(content, line)
	runes := []rune(text)
	if col > len(runes) {
		col = len(runes)
	}
	start := col
	for start > 0 && isIdentRune(runes[start-1]) {
		start--
	}
	end := col
	for end < len(runes) && isIdentRune(runes[end]) {
		end++
	}
	if start >= end {
		return "", false
	}
	isMember = start > 0 && runes[start-1] == '.'
	return string(runes[start:end]), isMember
}

// memberContext describes a member-access completion site: the receiver chain of identifiers
// before the cursor's '.', and the partial member name already typed after it.
//
// For `foo.bar.ba|` it returns chain=["foo","bar"], partial="ba", ok=true.
// For `foo.|`         it returns chain=["foo"],        partial="",   ok=true.
// For a non-member position it returns ok=false.
type memberContext struct {
	chain   []string
	partial string
}

// parseMemberContext inspects the text before the cursor and, if the cursor is at a member
// access (`receiver.partial`), returns the receiver chain and the partial member name. It only
// resolves plain identifier chains (a, a.b, a.b.c); if a call/index/paren interrupts the chain
// it stops, returning whatever prefix it could read (which may be empty → ok=false).
func parseMemberContext(content string, line, col int) (memberContext, bool) {
	prefix := runePrefix(lineAt(content, line), col)
	runes := []rune(prefix)
	i := len(runes)

	// Read the partial member identifier already typed after the dot.
	partialEnd := i
	for i > 0 && isIdentRune(runes[i-1]) {
		i--
	}
	partial := string(runes[i:partialEnd])

	// The character before the partial must be a '.' for this to be member access.
	if i == 0 || runes[i-1] != '.' {
		return memberContext{}, false
	}
	i-- // consume '.'

	// Walk the receiver chain backwards: identifier ('.' identifier)*.
	var chain []string
	for {
		segEnd := i
		for i > 0 && isIdentRune(runes[i-1]) {
			i--
		}
		if i == segEnd {
			// No identifier here (e.g. preceded by ')' or ']'); chain is broken.
			break
		}
		chain = append([]string{string(runes[i:segEnd])}, chain...)
		if i > 0 && runes[i-1] == '.' {
			i--
			continue
		}
		break
	}

	if len(chain) == 0 {
		return memberContext{}, false
	}
	return memberContext{chain: chain, partial: partial}, true
}

// --- AST-based cursor resolution (preferred over the text helpers above) ---
//
// Hover and go-to-definition act on tokens the user has already typed, so the parsed AST is
// authoritative there: ast.NodeAt maps the cursor to the exact identifier/member node, which is
// robust across line breaks, comments, and receiver chains the text scanner cannot follow.
// (Completion and signature help still scan text/tokens on purpose — they must work at a broken
// AST like `obj.` or `f(a, ` where the cursor sits in a syntax hole.)

// zeusPos converts an LSP position (0-based) to a Zeus source position (1-based).
func zeusPos(position protocol.Position) token.Position {
	return token.Position{Line: int(position.Line) + 1, Column: int(position.Character) + 1}
}

// identifierAt returns the identifier under pos and, when that identifier is the property of a
// member access (`receiver.ident`), the enclosing member-access node. Both are nil when the
// cursor is not on an identifier.
func identifierAt(program *ast.ProgramNode, pos token.Position) (*ast.IdentifierExprNode, *ast.ObjectPropertyAccessExprNode) {
	path := ast.NodeAt(program, pos)
	if len(path) == 0 {
		return nil, nil
	}
	id, ok := path[0].(*ast.IdentifierExprNode)
	if !ok {
		return nil, nil
	}
	// The parent is the member access only when the identifier is its property, not its receiver
	// (hovering the `a` in `a.b` is a plain reference to `a`, so path[1].Property != id there).
	if len(path) >= 2 {
		if member, ok := path[1].(*ast.ObjectPropertyAccessExprNode); ok && member.Property == id {
			return id, member
		}
	}
	return id, nil
}

// receiverChainFromExpr reconstructs a plain identifier chain (a, a.b, a.b.c) from a receiver
// expression so it can be resolved by resolveReceiverChain. It returns ok=false when the chain is
// interrupted by anything else (a call, an index, a parenthesized expression) — those receiver
// types cannot be resolved without an expression-type map (Phase 2).
func receiverChainFromExpr(expr ast.ExprNode) ([]string, bool) {
	switch e := expr.(type) {
	case *ast.IdentifierExprNode:
		// Parser error recovery can leave a typed-nil node or a node with a nil Name token.
		if e == nil || e.Name == nil {
			return nil, false
		}
		return []string{e.Name.Value}, true
	case *ast.ObjectPropertyAccessExprNode:
		if e == nil || e.Property == nil || e.Property.Name == nil {
			return nil, false
		}
		base, ok := receiverChainFromExpr(e.Object)
		if !ok {
			return nil, false
		}
		return append(base, e.Property.Name.Value), true
	default:
		return nil, false
	}
}

// resolveReceiver resolves the receiver expression of a member access to the class whose members
// should be offered. It first tries the plain identifier-chain path (which distinguishes static
// vs instance receivers), then falls back to the receiver's recorded expression type — so
// receivers that are calls or index expressions (`foo().bar`, `arr[0].baz`) resolve too.
func resolveReceiver(docInfo *DocumentInfo, objExpr ast.ExprNode) (receiver, bool) {
	// Scope-correct: an identifier receiver with a recorded binding resolves via that binding, so a
	// shadowed local is honoured rather than a same-named symbol from the flat table.
	if id, ok := objExpr.(*ast.IdentifierExprNode); ok {
		if sym, ok := docInfo.Semantic.SymbolAt(id); ok {
			if r, ok := receiverFromSymbol(sym); ok {
				return r, true
			}
		}
	}
	// Identifier chains (including static `Class.x` and `a.b.c`) via the symbol table.
	if docInfo.IRModule != nil {
		if chain, ok := receiverChainFromExpr(objExpr); ok {
			if r, ok := resolveReceiverChain(docInfo.IRModule, chain); ok {
				return r, true
			}
		}
	}
	// Non-identifier receiver (a call, an index): resolve via the recorded expression type.
	if t, ok := docInfo.Semantic.TypeAt(objExpr); ok {
		if r, ok := receiverFromValueType(t); ok {
			return r, true
		}
	}
	return receiver{}, false
}

// symbolByName finds a symbol by its source name. GetAllSymbols is keyed by name across all
// scopes (a locally-declared variable is therefore found too); temporary IR variables are
// never user-referenceable, so they are ignored.
func symbolByName(irModule *ir.IRModule, name string) zeus_value.Value {
	if isInternalSymbolName(name) {
		return nil
	}
	value, ok := irModule.GetAllSymbols()[name]
	if !ok {
		return nil
	}
	if v := zeus_value.AsVar(value); v != nil && v.IsTempVariable() {
		return nil
	}
	return value
}

// receiver is the resolved target of a member access. It is a class (instance or, for `Class.x`,
// static members) OR an interface (a value typed as an interface exposes that interface's members).
type receiver struct {
	class      *zeus_value.Class
	iface      *zeus_value.Interface
	isInstance bool
}

// classOfType returns the class backing an object type (obj instances), or nil for
// non-object types (primitives, functions, etc.) which have no member namespace.
func classOfType(t zeus_value.ValueType) *zeus_value.Class {
	if objType := zeus_value.AsObjectType(t); objType != nil {
		return objType.Class
	}
	return nil
}

// interfaceOfType returns the interface backing an interface type, or nil otherwise.
func interfaceOfType(t zeus_value.ValueType) *zeus_value.Interface {
	if it := zeus_value.AsInterfaceType(t); it != nil {
		return it.Interface
	}
	return nil
}

// receiverFromValueType resolves a value type to the receiver whose members it exposes: an object
// type yields its class, an interface type yields its interface. Both are instance receivers.
func receiverFromValueType(t zeus_value.ValueType) (receiver, bool) {
	if class := classOfType(t); class != nil {
		return receiver{class: class, isInstance: true}, true
	}
	if iface := interfaceOfType(t); iface != nil {
		return receiver{iface: iface, isInstance: true}, true
	}
	return receiver{}, false
}

// receiverFromSymbol maps a resolved symbol to a receiver: a class name resolves to a static
// receiver; a variable/object/ref-cell resolves to an instance receiver of its class or interface
// type.
func receiverFromSymbol(sym zeus_value.Value) (receiver, bool) {
	if sym == nil {
		return receiver{}, false
	}
	if class := zeus_value.AsClass(sym); class != nil {
		return receiver{class: class, isInstance: false}, true
	}
	if v := zeus_value.AsVar(sym); v != nil {
		return receiverFromValueType(v.ValueType)
	}
	if o := zeus_value.AsObject(sym); o != nil {
		return receiverFromValueType(o.ValueType)
	}
	// Escaped local captured as a ref cell.
	if rc := zeus_value.AsRefCellVar(sym); rc != nil {
		return receiverFromValueType(rc.ValueType)
	}
	return receiver{}, false
}

// resolveReceiverBase maps the first identifier of a receiver chain (by name) to a receiver.
func resolveReceiverBase(irModule *ir.IRModule, name string) (receiver, bool) {
	return receiverFromSymbol(symbolByName(irModule, name))
}

// memberKind distinguishes the three flavours of class member the LSP resolves.
type memberKind int

const (
	memberProperty memberKind = iota
	memberAccessor
	memberMethod
)

// member is a resolved class member in a single shape, so completion, hover, definition,
// member-type resolution, and signature help can all share one traversal instead of each
// re-walking the class hierarchy. valueType is the member's value type for chaining: a
// property's type, an accessor's value type, or a method's return type.
type member struct {
	kind       memberKind
	name       string
	ownerName  string       // source name of the class or interface that declares this member
	access     *token.Token // access modifier token (nil means the default, public)
	isStatic   bool
	isReadonly bool                      // properties only
	valueType  zeus_value.ValueType      // property/accessor value type, or method return type
	span       *token.Span               // declaration span (nil when unavailable)
	fn         *zeus_value.Function      // methods, and an accessor's getter
	accessor   *zeus_value.ClassAccessor // accessors only (nil otherwise)
}

// isPublic reports whether the member is publicly accessible.
func (m member) isPublic() bool { return isPublicModifier(m.access) }

// eachMember visits every user-facing member of the receiver, honoring the static/instance
// context. For a class it walks the class then its ancestors so a derived member is visited before
// (and thus shadows) a same-named base member; for an interface it walks the interface and its
// extended interfaces. Each name is visited at most once. Compiler-internal methods (the
// constructor, synthesized accessor methods, and the functor `__call__`) are not members.
func eachMember(r receiver, visit func(member)) {
	seen := map[string]bool{}
	emit := func(m member) {
		if seen[m.name] {
			return
		}
		seen[m.name] = true
		visit(m)
	}

	if r.iface != nil {
		eachInterfaceMember(r.iface, emit, map[int]bool{})
		return
	}

	wantStatic := !r.isInstance
	for cur := r.class; cur != nil; cur = cur.ParentClass {
		for _, prop := range cur.Properties {
			if prop.IsStatic != wantStatic {
				continue
			}
			emit(member{
				kind: memberProperty, name: prop.Property.Name, ownerName: cur.SourceName(),
				access: prop.AccessModifier, isStatic: prop.IsStatic, isReadonly: prop.IsReadonly,
				valueType: prop.Property.ValueType, span: prop.Property.Span,
			})
		}
		for _, acc := range cur.Accessors {
			if acc.IsStatic != wantStatic {
				continue
			}
			var valueType zeus_value.ValueType
			var span *token.Span
			if acc.Getter != nil {
				valueType, span = acc.Getter.ReturnType, acc.Getter.Span
			} else if acc.Setter != nil {
				if len(acc.Setter.Params) > 0 {
					valueType = acc.Setter.Params[0].ValueType
				}
				span = acc.Setter.Span
			}
			emit(member{
				kind: memberAccessor, name: acc.Name, ownerName: cur.SourceName(),
				access: acc.AccessModifier, isStatic: acc.IsStatic,
				valueType: valueType, span: span, fn: acc.Getter, accessor: acc,
			})
		}
		for _, m := range cur.Methods {
			name := m.Method.SourceName()
			if m.IsStatic != wantStatic {
				continue
			}
			if name == token.CONSTRUCTOR_METHOD_NAME || m.IsAccessor || name == token.FUNCTOR_CALL_METHOD_NAME {
				continue
			}
			emit(member{
				kind: memberMethod, name: name, ownerName: cur.SourceName(),
				access: m.AccessModifier, isStatic: m.IsStatic,
				valueType: m.Method.ReturnType, span: m.Method.Span, fn: m.Method,
			})
		}
	}
}

// eachInterfaceMember emits every member of an interface and its extended interfaces (the
// structural union), deduping by name through emit's shared seen-set. Interface members are always
// public and non-static; methods carry a signature but no body. visited guards against cyclic
// `extends` chains.
func eachInterfaceMember(iface *zeus_value.Interface, emit func(member), visited map[int]bool) {
	if iface == nil || visited[iface.Id] {
		return
	}
	visited[iface.Id] = true
	for _, prop := range iface.Properties {
		emit(member{
			kind: memberProperty, name: prop.Property.Name, ownerName: iface.Name,
			isReadonly: prop.IsReadonly, valueType: prop.Property.ValueType, span: prop.Property.Span,
		})
	}
	for _, fn := range iface.Methods {
		emit(member{
			kind: memberMethod, name: fn.SourceName(), ownerName: iface.Name,
			valueType: fn.ReturnType, span: fn.Span, fn: fn,
		})
	}
	for _, parent := range iface.Parents {
		eachInterfaceMember(parent, emit, visited)
	}
}

// lookupMember finds a member by name on the receiver, returning the most-derived match.
func lookupMember(r receiver, name string) (member, bool) {
	var found member
	ok := false
	eachMember(r, func(m member) {
		if !ok && m.name == name {
			found, ok = m, true
		}
	})
	return found, ok
}

// memberType returns the value type of a named member (for resolving member-access chains).
func memberType(r receiver, name string) (zeus_value.ValueType, bool) {
	m, ok := lookupMember(r, name)
	if !ok {
		return nil, false
	}
	return m.valueType, true
}

// resolveReceiverChain walks a full receiver chain (e.g. ["a","b","c"]) to the receiver whose
// members should be offered. Only the trailing element's members are listed; every element
// before it must resolve to an object-typed member so the walk can continue.
func resolveReceiverChain(irModule *ir.IRModule, chain []string) (receiver, bool) {
	if len(chain) == 0 {
		return receiver{}, false
	}
	r, ok := resolveReceiverBase(irModule, chain[0])
	if !ok {
		return receiver{}, false
	}
	for _, name := range chain[1:] {
		t, ok := memberType(r, name)
		if !ok {
			return receiver{}, false
		}
		// A member access always yields an instance of the member's type (class or interface).
		next, ok := receiverFromValueType(t)
		if !ok {
			return receiver{}, false
		}
		r = next
	}
	return r, true
}

// isPublicModifier reports whether an access modifier grants public access. A nil modifier
// is the default, which is public.
func isPublicModifier(tok *token.Token) bool {
	return tok == nil || tok.Type == token.TokenTypePublic
}

// accessModifierString renders an access modifier token as a source keyword ("" for public/nil).
func accessModifierString(tok *token.Token) string {
	if tok == nil {
		return ""
	}
	switch tok.Type {
	case token.TokenTypePrivate:
		return "private "
	case token.TokenTypeProtected:
		return "protected "
	}
	return ""
}

// funcSignature renders a function's parameter list and return type, e.g. "(x: i32): f64".
func funcSignature(fn *zeus_value.Function) string {
	params := make([]string, 0, len(fn.Params))
	for _, p := range fn.Params {
		if p.ValueType != nil {
			params = append(params, p.Name+": "+p.ValueType.String())
		} else {
			params = append(params, p.Name)
		}
	}
	ret := "void"
	if fn.ReturnType != nil {
		ret = fn.ReturnType.String()
	}
	return "(" + strings.Join(params, ", ") + "): " + ret
}

// typeString renders a value type for display, tolerating nil.
func typeString(t zeus_value.ValueType) string {
	if t == nil {
		return "unknown"
	}
	return t.String()
}
