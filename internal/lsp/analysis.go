package lsp

import (
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// This file contains the position/text analysis and symbol-resolution helpers shared
// by completion and hover. Resolution is deliberately text-driven (it inspects the line
// around the cursor) and then answers questions against the IR symbol table, which — after
// parseDocument runs the type checker — holds every declared symbol with a resolved type
// (Walk visits all scopes, so locals are included, not just globals).

// isIdentRune reports whether r can appear in a Zeus identifier.
func isIdentRune(r rune) bool {
	return r == '_' || r == '$' ||
		(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
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
		return []string{e.Name.Value}, true
	case *ast.ObjectPropertyAccessExprNode:
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
		if class := classOfType(t); class != nil {
			return receiver{class: class, isInstance: true}, true
		}
	}
	return receiver{}, false
}

// symbolByName finds a symbol by its source name. GetAllSymbols is keyed by name across all
// scopes (a locally-declared variable is therefore found too); temporary IR variables are
// never user-referenceable, so they are ignored.
func symbolByName(irModule *ir.IRModule, name string) zeus_value.Value {
	value, ok := irModule.GetAllSymbols()[name]
	if !ok {
		return nil
	}
	if v := zeus_value.AsVar(value); v != nil && v.IsTempVariable() {
		return nil
	}
	return value
}

// receiver is the resolved target of a member access: a class, plus whether the access is on
// an instance (obj.x → instance members) or on the class itself (Class.x → static members).
type receiver struct {
	class      *zeus_value.Class
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

// receiverFromSymbol maps a resolved symbol to a receiver: a class resolves to a static receiver;
// a variable/object/ref-cell of object type resolves to an instance receiver.
func receiverFromSymbol(sym zeus_value.Value) (receiver, bool) {
	if sym == nil {
		return receiver{}, false
	}
	if class := zeus_value.AsClass(sym); class != nil {
		return receiver{class: class, isInstance: false}, true
	}
	if v := zeus_value.AsVar(sym); v != nil {
		if class := classOfType(v.ValueType); class != nil {
			return receiver{class: class, isInstance: true}, true
		}
	}
	if o := zeus_value.AsObject(sym); o != nil {
		if class := classOfType(o.ValueType); class != nil {
			return receiver{class: class, isInstance: true}, true
		}
	}
	// Escaped local captured as a ref cell.
	if rc := zeus_value.AsRefCellVar(sym); rc != nil {
		if class := classOfType(rc.ValueType); class != nil {
			return receiver{class: class, isInstance: true}, true
		}
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
	owner      *zeus_value.Class // the class that declares this member
	access     *token.Token      // access modifier token (nil means the default, public)
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
// context. It walks the class then its ancestors so a derived member is visited before (and
// thus shadows) a same-named base member; each name is visited at most once. Compiler-internal
// methods (the constructor, synthesized accessor methods, and the functor `__call__`) are not
// members and are skipped.
func eachMember(r receiver, visit func(member)) {
	seen := map[string]bool{}
	emit := func(m member) {
		if seen[m.name] {
			return
		}
		seen[m.name] = true
		visit(m)
	}

	wantStatic := !r.isInstance
	for cur := r.class; cur != nil; cur = cur.ParentClass {
		for _, prop := range cur.Properties {
			if prop.IsStatic != wantStatic {
				continue
			}
			emit(member{
				kind: memberProperty, name: prop.Property.Name, owner: cur,
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
				kind: memberAccessor, name: acc.Name, owner: cur,
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
				kind: memberMethod, name: name, owner: cur,
				access: m.AccessModifier, isStatic: m.IsStatic,
				valueType: m.Method.ReturnType, span: m.Method.Span, fn: m.Method,
			})
		}
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
		class := classOfType(t)
		if class == nil {
			return receiver{}, false
		}
		// A member access always yields an instance of the member's type.
		r = receiver{class: class, isInstance: true}
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
