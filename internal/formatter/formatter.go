// Package formatter implements a Prettier-style source formatter for Zeus. It
// parses source to an AST and re-prints it from an intermediate Doc IR whose
// printer fits each construct to a configurable print width. Comments captured by
// the lexer (via Lexer.Comments()) are re-attached to nodes by position. See
// wiki/formatter.md.
package formatter

import (
	"sort"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ast"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// Format renders a parsed program to canonical Zeus source using opts, weaving in
// the given comments (from Lexer.Comments()) at their source positions. The result
// always ends with exactly one trailing newline (or is empty for an empty program
// with no comments).
func Format(program *ast.ProgramNode, comments []*token.Comment, opts Options) string {
	sorted := append([]*token.Comment(nil), comments...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return posLess(sorted[i].Span.Start, sorted[j].Span.Start)
	})

	f := &formatter{opts: opts, comments: sorted}
	doc, _ := f.printBody(f.stmtItems(program.Statements), nil)
	out := printDoc(doc, opts)
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return ""
	}
	return out + "\n"
}

type formatter struct {
	opts        Options
	comments    []*token.Comment
	nextComment int // index of the next not-yet-emitted comment
}

// bodyItem is one printable statement or class member together with its source
// span, used to interleave comments and preserve blank lines.
type bodyItem struct {
	span   *token.Span
	render func() Doc
}

func (f *formatter) stmtItems(stmts []ast.StmtNode) []bodyItem {
	items := make([]bodyItem, 0, len(stmts))
	for _, s := range stmts {
		s := s
		items = append(items, bodyItem{span: s.GetSpan(), render: func() Doc { return f.printStmt(s) }})
	}
	return items
}

// printBody renders a sequence of statements/members, interleaving leading,
// trailing, and dangling comments and preserving single blank lines. contentEnd
// bounds dangling comments (nil = consume all remaining, used for the program).
// It returns the rendered doc and whether anything (item or comment) was emitted.
func (f *formatter) printBody(items []bodyItem, contentEnd *token.Position) (Doc, bool) {
	parts := []Doc{}
	prevEndLine := -1

	emit := func(doc Doc, startLine, endLine int) {
		if len(parts) > 0 {
			parts = append(parts, hardline)
			if startLine-prevEndLine > 1 {
				parts = append(parts, hardline) // preserve one blank line
			}
		}
		parts = append(parts, doc)
		prevEndLine = endLine
	}

	for idx, item := range items {
		if item.span == nil {
			emit(item.render(), prevEndLine, prevEndLine)
			continue
		}

		// Leading own-line comments (trailing comments of the previous item were
		// already consumed, so anything remaining before this item leads it).
		for f.nextComment < len(f.comments) {
			c := f.comments[f.nextComment]
			if !posLess(c.Span.Start, item.span.Start) {
				break
			}
			f.nextComment++
			emit(commentDoc(c), c.Span.Start.Line, c.Span.End.Line)
		}

		// The item itself; rendering recurses into nested blocks, consuming any
		// comments inside them first.
		doc := item.render()
		endLine := item.span.End.Line

		// Trailing comment(s) on the item's last line, but only up to the next
		// item's start — so a comment after a later same-line statement isn't
		// mis-attached to this one.
		var nextStart *token.Position
		if idx+1 < len(items) && items[idx+1].span != nil {
			nextStart = &items[idx+1].span.Start
		} else {
			nextStart = contentEnd
		}
		for f.nextComment < len(f.comments) {
			c := f.comments[f.nextComment]
			if c.Span.Start.Line != item.span.End.Line || posLess(c.Span.Start, item.span.End) {
				break
			}
			if nextStart != nil && !posLess(c.Span.Start, *nextStart) {
				break
			}
			f.nextComment++
			doc = concat(doc, text(" "), commentDoc(c))
			endLine = c.Span.End.Line
		}

		emit(doc, item.span.Start.Line, endLine)
	}

	// Dangling comments before the container's closing brace / end of file.
	for f.nextComment < len(f.comments) {
		c := f.comments[f.nextComment]
		if contentEnd != nil && !posLess(c.Span.Start, *contentEnd) {
			break
		}
		f.nextComment++
		emit(commentDoc(c), c.Span.Start.Line, c.Span.End.Line)
	}

	return concat(parts...), len(parts) > 0
}

// posLess reports whether a comes strictly before b in the source.
func posLess(a, b token.Position) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Column < b.Column
}

// commentDoc renders a single comment. A JSDoc-style block comment (every
// continuation line starts with `*`) is re-indented to the current level with the
// `*` aligned. Any other multi-line block comment is emitted verbatim (via
// literalLine) so tables, ASCII art, and indented content keep their layout.
func commentDoc(c *token.Comment) Doc {
	if !c.IsBlock || !strings.Contains(c.Text, "\n") {
		return text(c.Text)
	}
	// Normalize CRLF so the comment splits cleanly regardless of source line endings.
	lines := strings.Split(strings.ReplaceAll(c.Text, "\r\n", "\n"), "\n")

	if isJSDoc(lines) {
		parts := []Doc{text(strings.TrimRight(lines[0], " \t"))}
		for _, line := range lines[1:] {
			parts = append(parts, hardline, text(" "+strings.TrimSpace(line)))
		}
		return concat(parts...)
	}

	parts := make([]Doc, 0, len(lines)*2)
	for i, line := range lines {
		if i > 0 {
			parts = append(parts, literalLine)
		}
		parts = append(parts, text(strings.TrimRight(line, " \t")))
	}
	return concat(parts...)
}

// isJSDoc reports whether a multi-line block comment is JSDoc-style, i.e. every
// line after the first begins with `*`.
func isJSDoc(lines []string) bool {
	if len(lines) < 2 {
		return false
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(strings.TrimSpace(line), "*") {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

// semi renders a statement/member terminator: a literal `;` when the Semicolons
// option is on, otherwise nothing. Callers must only use it where Zeus's automatic
// semicolon insertion makes the `;` redundant (i.e. the preceding token is a
// value-ending token — see lexer.shouldInsertSemicolon). Keyword-terminated
// statements like a bare `return`/`break`/`continue` keep an explicit `;`.
func (f *formatter) semi() Doc {
	if f.opts.Semicolons {
		return text(";")
	}
	return nilDoc
}

func (f *formatter) printStmt(stmt ast.StmtNode) Doc {
	switch s := stmt.(type) {
	case *ast.ExprStmtNode:
		d := f.printExpr(s.Expr)
		if !exprEndsWithBlock(s.Expr) {
			d = concat(d, f.semi())
		}
		return d

	case *ast.VarDeclStmtNode:
		return f.printVarDeclStmt(s, true)

	case *ast.BlockStmtNode:
		return f.printBlockFor(s)

	case *ast.ReturnStmtNode:
		if s.Expr == nil {
			// Bare `return` ends in a keyword — ASI never inserts here, so the `;` stays.
			return text("return;")
		}
		return concat(text("return "), f.printExpr(s.Expr), f.semi())

	case *ast.IfStmtNode:
		return f.printIf(s)

	case *ast.WhileStmtNode:
		// Unlike `if`/`for`, the while parser treats the parentheses as a grouping
		// expression rather than consuming them, so strip one grouping layer before
		// re-adding canonical parens (otherwise they double up).
		return concat(text("while "), f.condParen(f.printExpr(unwrapGrouping(s.Condition))), text(" "), f.printBlockOf(s.Body))

	case *ast.ForStmtNode:
		return f.printFor(s)

	case *ast.ImportStmtNode:
		return f.printImport(s)

	case *ast.ExportStmtNode:
		d := concat(text("export "), f.printExpr(s.Expr))
		if !exprEndsWithBlock(s.Expr) {
			d = concat(d, f.semi())
		}
		return d

	case *ast.TryCatchStmtNode:
		return f.printTryCatch(s)

	case *ast.ThrowStmtNode:
		return concat(text("throw "), f.printExpr(s.Expr), f.semi())

	case *ast.BreakStmtNode:
		// `break`/`continue` end in a keyword — ASI never inserts here, so the `;` stays.
		return text("break;")

	case *ast.ContinueStmtNode:
		return text("continue;")

	case *ast.AnnotationStmtNode:
		return concat(text(s.Annotation.PrettyString()), f.semi())
	}

	panic("formatter: unhandled statement node")
}

func (f *formatter) printVarDeclStmt(s *ast.VarDeclStmtNode, semicolon bool) Doc {
	if len(s.Decls) == 0 {
		return text("")
	}
	keyword := s.Decls[0].DeclType.String()
	declDocs := make([]Doc, 0, len(s.Decls))
	for _, decl := range s.Decls {
		declDocs = append(declDocs, f.printVarDecl(decl))
	}
	d := concat(text(keyword+" "), join(text(", "), declDocs))
	if semicolon {
		last := s.Decls[len(s.Decls)-1]
		// `let x: T` ends in a type and `let x = expr` in the initializer's value token — after
		// either, ASI inserts the terminator, so it can be omitted (unless the initializer is a
		// self-terminating block).
		if last.Initializer == nil || !exprEndsWithBlock(last.Initializer) {
			d = concat(d, f.semi())
		}
	}
	return d
}

// printVarDecl prints a single declaration without its let/const keyword:
// `[...]name[: T][ = init]`.
func (f *formatter) printVarDecl(decl ast.VarDeclNode) Doc {
	parts := []Doc{}
	if decl.IsVariadic {
		parts = append(parts, text("..."))
	}
	parts = append(parts, text(decl.Identifier.Name.Value))
	if decl.ValueType != nil && decl.ValueType.ValueType != nil {
		parts = append(parts, text(": "+formatType(decl.ValueType.ValueType)))
	}
	if decl.Initializer != nil {
		parts = append(parts, text(" = "), f.printExpr(decl.Initializer))
	}
	return concat(parts...)
}

// printBlock renders a `{ ... }` block. contentEnd is the position of the closing
// brace, bounding which dangling comments belong inside the block.
func (f *formatter) printBlock(stmts []ast.StmtNode, contentEnd *token.Position) Doc {
	body, has := f.printBody(f.stmtItems(stmts), contentEnd)
	if !has {
		return text("{}")
	}
	return concat(
		text("{"),
		indent(concat(hardline, body)),
		hardline,
		text("}"),
	)
}

// printBlockFor renders a BlockStmtNode, using its span end as the brace bound.
func (f *formatter) printBlockFor(block *ast.BlockStmtNode) Doc {
	return f.printBlock(block.Statements, &block.Span.End)
}

// printBlockOf renders a statement as a brace-delimited block, wrapping a bare
// (non-block) statement in braces so control-flow bodies always use braces.
func (f *formatter) printBlockOf(stmt ast.StmtNode) Doc {
	if block, ok := stmt.(*ast.BlockStmtNode); ok {
		return f.printBlockFor(block)
	}
	end := stmt.GetSpan().End
	return f.printBlock([]ast.StmtNode{stmt}, &end)
}

func (f *formatter) printIf(s *ast.IfStmtNode) Doc {
	d := concat(text("if "), f.condParen(f.printExpr(s.Condition)), text(" "), f.printBlockOf(s.ThenStmt))
	if s.ElseStmt != nil {
		if elseIf, ok := s.ElseStmt.(*ast.IfStmtNode); ok {
			d = concat(d, text(" else "), f.printIf(elseIf))
		} else {
			d = concat(d, text(" else "), f.printBlockOf(s.ElseStmt))
		}
	}
	return d
}

func (f *formatter) printFor(s *ast.ForStmtNode) Doc {
	init := nilDoc
	if s.Init != nil {
		init = f.printForInit(s.Init)
	}
	cond := nilDoc
	if s.Condition != nil {
		cond = f.printExpr(s.Condition)
	}
	update := nilDoc
	if s.Update != nil {
		update = f.printExpr(s.Update)
	}
	header := concat(text("for ("), init, text("; "), cond, text("; "), update, text(") "))
	return concat(header, f.printBlockOf(s.Body))
}

// printForInit renders a for-loop initializer inline, without a trailing semicolon.
func (f *formatter) printForInit(stmt ast.StmtNode) Doc {
	switch s := stmt.(type) {
	case *ast.VarDeclStmtNode:
		return f.printVarDeclStmt(s, false)
	case *ast.ExprStmtNode:
		return f.printExpr(s.Expr)
	}
	return f.printStmt(stmt)
}

func (f *formatter) printImport(s *ast.ImportStmtNode) Doc {
	names := make([]Doc, 0, len(s.Imports))
	for _, imp := range s.Imports {
		names = append(names, text(imp.Name.Value))
	}
	return group(concat(
		text("import {"),
		indent(concat(line, join(concat(text(","), line), names))),
		ifBreak(text(","), nilDoc),
		line,
		text("} from "),
		text("\""+s.Source.Value+"\";"),
	))
}

func (f *formatter) printTryCatch(s *ast.TryCatchStmtNode) Doc {
	d := concat(text("try "), f.printBlockFor(s.TryBody))
	for _, clause := range s.CatchClauses {
		d = concat(d,
			text(" catch ("),
			text(clause.ErrorVar.Name.Value),
			text(": "+formatType(clause.ErrorType.ValueType)),
			text(") "),
			f.printBlockFor(clause.Body),
		)
	}
	if s.FinallyBody != nil {
		d = concat(d, text(" finally "), f.printBlockFor(s.FinallyBody))
	}
	return d
}

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

func (f *formatter) printExpr(expr ast.ExprNode) Doc {
	switch e := expr.(type) {
	case *ast.NumberExprNode:
		return text(e.Value.Value)
	case *ast.BooleanExprNode:
		// Boolean tokens are keywords with empty Value; use the token spelling.
		return text(e.Value.Type.String())
	case *ast.CharExprNode:
		return text("'" + escapeChar(e.Value.Value) + "'")
	case *ast.StringConstantExprNode:
		return text("\"" + escapeString(e.Value.Value) + "\"")
	case *ast.IdentifierExprNode:
		return text(e.Name.Value)
	case *ast.NullExprNode:
		return text("null")
	case *ast.GroupingExprNode:
		return concat(text("("), f.printExpr(e.Expr), text(")"))
	case *ast.BinaryExprNode:
		return concat(f.printExpr(e.Left), text(" "+e.Operator.Type.String()+" "), f.printExpr(e.Right))
	case *ast.UnaryExprNode:
		return concat(text(e.Operator.Type.String()), f.printExpr(e.Expr))
	case *ast.PostfixExprNode:
		return concat(f.printExpr(e.Expr), text(e.Operator.Type.String()))
	case *ast.FunctionCallExprNode:
		return concat(f.printExpr(e.Callee), f.argList(e.Params))
	case *ast.NewExprNode:
		// Array allocations (`new i32[10]`, `new Point[]`) have an indexing callee and
		// take no argument parentheses; constructor calls (`new Foo(a)`) always do.
		if _, ok := e.Callee.(*ast.IndexingExprNode); ok {
			return concat(text("new "), f.printExpr(e.Callee))
		}
		return concat(text("new "), f.printExpr(e.Callee), f.argList(e.Args))
	case *ast.IndexingExprNode:
		return f.printIndexing(e)
	case *ast.ArrayLiteralExprNode:
		items := make([]Doc, 0, len(e.Elements))
		for _, el := range e.Elements {
			items = append(items, f.printExpr(el))
		}
		return f.delimitedList("[", "]", items)
	case *ast.ObjectPropertyAccessExprNode:
		return concat(f.printExpr(e.Object), text("."+e.Property.Name.Value))
	case *ast.TernaryExprNode:
		return concat(f.printExpr(e.Condition), text(" ? "), f.printExpr(e.Then), text(" : "), f.printExpr(e.Else))
	case *ast.CastExprNode:
		return concat(f.printExpr(e.Expr), text(" as "+formatType(e.AsType.ValueType)))
	case *ast.TemplateStringExprNode:
		return f.printTemplate(e)
	case *ast.FunctionDeclExprNode:
		return f.printFunction(e)
	case *ast.ClassDeclExprNode:
		return f.printClass(e)
	case *ast.InterfaceDeclExprNode:
		return f.printInterface(e)
	case *ast.ValueTypeNode:
		return text(formatType(e.ValueType))
	}

	panic("formatter: unhandled expression node")
}

func (f *formatter) printIndexing(e *ast.IndexingExprNode) Doc {
	parts := []Doc{f.printExpr(e.Array)}
	for _, idx := range e.IndexingMeta.IndexingExprs {
		if idx == nil {
			parts = append(parts, text("[]"))
			continue
		}
		parts = append(parts, text("["), f.printExpr(idx), text("]"))
	}
	return concat(parts...)
}

func (f *formatter) printTemplate(e *ast.TemplateStringExprNode) Doc {
	parts := []Doc{text("`")}
	for _, p := range e.Parts {
		if p.IsExpr {
			parts = append(parts, text("${"), f.printExpr(p.Expr), text("}"))
		} else {
			parts = append(parts, text(escapeTemplate(p.Str)))
		}
	}
	parts = append(parts, text("`"))
	return concat(parts...)
}

// externCallText reconstructs the @extern(...) decorator for a binding:
//   direct C-ABI to a zeus_ runtime symbol -> @extern("zeus", "<rest>")
//   direct C-ABI to any other C symbol     -> @extern("C", "<symbol>")
//   fat-ABI Zig runtime symbol             -> @extern("<symbol>")
func externCallText(symbol string, isCExtern bool) string {
	if isCExtern {
		if strings.HasPrefix(symbol, "zeus_") {
			return "@extern(\"zeus\", \"" + strings.TrimPrefix(symbol, "zeus_") + "\")"
		}
		return "@extern(\"C\", \"" + symbol + "\")"
	}
	return "@extern(\"" + symbol + "\")"
}

func (f *formatter) printFunction(e *ast.FunctionDeclExprNode) Doc {
	// Anonymous functions come from fat-arrow syntax (`(params) => { body }`) and
	// are rendered back as arrows; named ones use the `function` keyword. The AST
	// does not otherwise distinguish the two forms.
	if e.Name == nil {
		arrow := concat(f.paramList(e.Params), f.returnType(e.ReturnType), text(" => "))
		if e.Body == nil {
			return concat(arrow, text("{}"))
		}
		return concat(arrow, f.printBlockFor(e.Body))
	}

	sig := concat(text("function "+e.Name.Name.Value), f.paramList(e.Params), f.returnType(e.ReturnType))
	if e.ExternSymbol != "" {
		return concat(text(externCallText(e.ExternSymbol, e.IsCExtern)+" "), sig, f.semi())
	}
	if e.Body == nil {
		return concat(sig, f.semi())
	}
	return concat(sig, text(" "), f.printBlockFor(e.Body))
}

func (f *formatter) printClass(e *ast.ClassDeclExprNode) Doc {
	head := "class"
	if e.Name != nil {
		head += " " + e.Name.Name.Value
	}
	if e.ParentClass != nil {
		head += " extends " + e.ParentClass.Name.Value
	}
	if len(e.Implements) > 0 {
		names := make([]string, len(e.Implements))
		for i, iface := range e.Implements {
			names[i] = iface.Name.Value
		}
		head += " implements " + strings.Join(names, ", ")
	}

	// Properties and methods are stored in separate slices; sort by span so the
	// original source order (and any interleaving) is preserved, and so comments
	// attach to the right member. Blank lines between members come from the source.
	items := make([]bodyItem, 0, len(e.Properties)+len(e.Methods))
	for _, p := range e.Properties {
		p := p
		items = append(items, bodyItem{span: p.Span, render: func() Doc { return f.printProperty(p) }})
	}
	for _, m := range e.Methods {
		m := m
		items = append(items, bodyItem{span: m.Span, render: func() Doc { return f.printMethod(m) }})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return posLess(items[i].span.Start, items[j].span.Start)
	})

	body, has := f.printBody(items, &e.Span.End)
	if !has {
		return concat(text(head+" "), text("{}"))
	}
	return concat(text(head+" "), text("{"), indent(concat(hardline, body)), hardline, text("}"))
}

func (f *formatter) printProperty(p *ast.ClassProperty) Doc {
	parts := []Doc{}
	if p.AccessModifier != nil {
		parts = append(parts, text(p.AccessModifier.Type.String()+" "))
	}
	if p.IsStatic {
		parts = append(parts, text("static "))
	}
	if p.IsReadonly {
		parts = append(parts, text("readonly "))
	}
	parts = append(parts, text(p.Name.Name.Value))
	if p.ValueType != nil && p.ValueType.ValueType != nil {
		parts = append(parts, text(": "+formatType(p.ValueType.ValueType)))
	}
	if p.Initializer != nil {
		parts = append(parts, text(" = "), f.printExpr(p.Initializer))
	}
	parts = append(parts, f.semi())
	return concat(parts...)
}

func (f *formatter) printMethod(m *ast.ClassMethod) Doc {
	parts := []Doc{}
	if m.ExternSymbol != "" {
		parts = append(parts, text(externCallText(m.ExternSymbol, false)+" "))
	}
	if m.AccessModifier != nil {
		parts = append(parts, text(m.AccessModifier.Type.String()+" "))
	}
	if m.IsStatic {
		parts = append(parts, text("static "))
	}
	switch m.Accessor {
	case ast.AccessorKindGetter:
		parts = append(parts, text("get "))
	case ast.AccessorKindSetter:
		parts = append(parts, text("set "))
	}
	parts = append(parts, text(m.Name.Name.Value), f.paramList(m.Params))

	// Return type: setters are always void; a bodied constructor omits it; every
	// other method (including extern constructors) prints its declared type.
	isConstructor := m.Name.Name.Value == token.CONSTRUCTOR_METHOD_NAME
	if m.Accessor == ast.AccessorKindSetter {
		parts = append(parts, text(": void"))
	} else if !(isConstructor && m.Body != nil) {
		parts = append(parts, f.returnType(m.ReturnType))
	}

	if m.Body == nil {
		parts = append(parts, f.semi())
	} else {
		parts = append(parts, text(" "), f.printBlockFor(m.Body))
	}
	return concat(parts...)
}

func (f *formatter) printInterface(e *ast.InterfaceDeclExprNode) Doc {
	head := "interface"
	if e.Name != nil {
		head += " " + e.Name.Name.Value
	}
	if len(e.Parents) > 0 {
		names := make([]string, len(e.Parents))
		for i, p := range e.Parents {
			names[i] = p.Name.Value
		}
		head += " extends " + strings.Join(names, ", ")
	}

	// Property and method signatures live in separate slices; sort by span to preserve the
	// original source order and comment attachment, mirroring printClass.
	items := make([]bodyItem, 0, len(e.Properties)+len(e.Methods))
	for _, p := range e.Properties {
		p := p
		items = append(items, bodyItem{span: p.Span, render: func() Doc { return f.printInterfaceProperty(p) }})
	}
	for _, m := range e.Methods {
		m := m
		items = append(items, bodyItem{span: m.Span, render: func() Doc { return f.printInterfaceMethod(m) }})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return posLess(items[i].span.Start, items[j].span.Start)
	})

	body, has := f.printBody(items, &e.Span.End)
	if !has {
		return concat(text(head+" "), text("{}"))
	}
	return concat(text(head+" "), text("{"), indent(concat(hardline, body)), hardline, text("}"))
}

func (f *formatter) printInterfaceProperty(p *ast.InterfacePropertySignature) Doc {
	parts := []Doc{}
	if p.IsReadonly {
		parts = append(parts, text("readonly "))
	}
	parts = append(parts, text(p.Name.Name.Value))
	if p.ValueType != nil && p.ValueType.ValueType != nil {
		parts = append(parts, text(": "+formatType(p.ValueType.ValueType)))
	}
	parts = append(parts, f.semi())
	return concat(parts...)
}

func (f *formatter) printInterfaceMethod(m *ast.InterfaceMethodSignature) Doc {
	parts := []Doc{text(m.Name.Name.Value), f.paramList(m.Params), f.returnType(m.ReturnType), f.semi()}
	return concat(parts...)
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func (f *formatter) returnType(vt *ast.ValueTypeNode) Doc {
	if vt == nil || vt.ValueType == nil {
		return nilDoc
	}
	return text(": " + formatType(vt.ValueType))
}

func (f *formatter) paramList(params []*ast.VarDeclNode) Doc {
	items := make([]Doc, 0, len(params))
	for _, p := range params {
		items = append(items, f.printVarDecl(*p))
	}
	return f.delimitedList("(", ")", items)
}

func (f *formatter) argList(args []ast.ExprNode) Doc {
	items := make([]Doc, 0, len(args))
	for _, a := range args {
		items = append(items, f.printExpr(a))
	}
	return f.delimitedList("(", ")", items)
}

// delimitedList renders open + comma-separated items + close as a group that
// breaks (one item per line, with a trailing comma) when it exceeds the width.
func (f *formatter) delimitedList(open, close string, items []Doc) Doc {
	if len(items) == 0 {
		return text(open + close)
	}
	return group(concat(
		text(open),
		indent(concat(softline, join(concat(text(","), line), items))),
		ifBreak(text(","), nilDoc),
		softline,
		text(close),
	))
}

// condParen wraps a control-flow condition in parentheses. It is intentionally
// NOT breakable: Zeus performs automatic semicolon insertion after a value token
// at end of line, so putting the closing `)` on its own line would make the last
// condition token line-final and insert a spurious `;`.
func (f *formatter) condParen(inner Doc) Doc {
	return concat(text("("), inner, text(")"))
}

// unwrapGrouping removes a single outer parenthesized-grouping layer, if present.
func unwrapGrouping(expr ast.ExprNode) ast.ExprNode {
	if g, ok := expr.(*ast.GroupingExprNode); ok {
		return g.Expr
	}
	return expr
}

// exprEndsWithBlock reports whether an expression statement ends with a block (or
// a self-terminating extern/`;`), meaning no extra semicolon should be appended.
// Mirrors the parser's helper of the same name.
func exprEndsWithBlock(expr ast.ExprNode) bool {
	switch e := expr.(type) {
	case *ast.FunctionDeclExprNode, *ast.ClassDeclExprNode, *ast.InterfaceDeclExprNode:
		return true
	case *ast.BinaryExprNode:
		return exprEndsWithBlock(e.Right)
	case *ast.TernaryExprNode:
		return exprEndsWithBlock(e.Else)
	}
	return false
}

// formatType renders a resolved value type back to Zeus source syntax. It cannot
// rely on ValueType.String() because BoolType stringifies as "bool" while the
// Zeus keyword is "boolean".
func formatType(vt zeus_value.ValueType) string {
	switch t := vt.(type) {
	case zeus_value.BoolType:
		return "boolean"
	case zeus_value.ArrayType:
		elem := formatType(t.ElementType)
		// A function-type element needs parentheses so `((i32) => i32)[]` is not
		// misread as a function returning `i32[]`.
		if _, isFn := t.ElementType.(zeus_value.FunctionType); isFn {
			elem = "(" + elem + ")"
		}
		return elem + "[]"
	case zeus_value.UserDefinedType:
		return t.Name
	case zeus_value.ObjectType:
		return t.Class.Name
	case zeus_value.ClassType:
		return t.Class.Name
	case zeus_value.FunctionType:
		paramTypes := make([]string, 0, len(t.ParamTypes))
		for _, pt := range t.ParamTypes {
			paramTypes = append(paramTypes, formatType(pt))
		}
		return "(" + strings.Join(paramTypes, ", ") + ") => " + formatType(t.ReturnType)
	default:
		// IntType, FloatType, VoidType, NullType already stringify to valid syntax.
		return vt.String()
	}
}

func escapeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func escapeChar(s string) string {
	switch s {
	case `\`:
		return `\\`
	case "'":
		return `\'`
	case "\n":
		return `\n`
	case "\t":
		return `\t`
	case "\r":
		return `\r`
	case "\x00":
		return `\0`
	}
	return s
}

// escapeTemplate escapes backslashes and backticks in a template literal's static
// text; embedded newlines are kept literal (they are valid inside backticks).
func escapeTemplate(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '`':
			b.WriteString("\\`")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
