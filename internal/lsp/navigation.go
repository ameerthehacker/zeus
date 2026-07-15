package lsp

import (
	"sort"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

// identifierOccurrences returns the spans of every identifier token named `name`. It lexes the
// document so matches inside string literals, comments, and language keywords are excluded
// (a purely textual search would wrongly match those). Resolution is name-based and
// single-file — good enough for document highlight and single-file references.
func identifierOccurrences(content, name string) []*token.Span {
	tokens, _ := lexer.NewLexer(content).Lex()
	var spans []*token.Span
	for _, tok := range tokens {
		if tok.Type == token.TokenTypeIdentifier && tok.Value == name {
			spans = append(spans, tok.Span)
		}
	}
	return spans
}

// occurrences returns the source spans of every occurrence of the thing under the cursor. When the
// cursor is on a resolved symbol it uses the binding index for a precise, scope-aware result (only
// the symbol actually in scope — a shadowed same-named identifier is not included); otherwise it
// falls back to a name-based token scan so members, keywords, and unresolved identifiers still
// highlight. The name-based scan is single-file only.
func (s *Server) occurrences(docInfo *DocumentInfo, position protocol.Position) []*token.Span {
	if docInfo.AST != nil {
		if id, _ := identifierAt(docInfo.AST, zeusPos(position)); id != nil {
			// Precise path: the binding index has all occurrences of the symbol — unless it is an
			// escape-boxed variable (see symbolIsFragmented), whose occurrences are split across
			// multiple ref-cell objects and would come back incomplete; for those we fall through
			// to the name scan, which is complete (if over-approximate) rather than missing uses.
			if sym, ok := docInfo.Semantic.SymbolAt(id); ok && !symbolIsFragmented(sym) {
				if nodes := docInfo.Semantic.SymbolUses(sym); len(nodes) > 0 {
					spans := make([]*token.Span, 0, len(nodes))
					for _, n := range nodes {
						spans = append(spans, n.GetSpan())
					}
					sortSpans(spans)
					return spans
				}
			}
		}
	}
	word, _ := wordAt(docInfo.Content, int(position.Line), int(position.Character))
	if word == "" {
		return nil
	}
	return identifierOccurrences(docInfo.Content, word)
}

// symbolIsFragmented reports whether a symbol's occurrences are split across multiple symbol
// objects in the binding index. Escape-boxed variables (*RefCellVar) are represented by a distinct
// ref cell in the declaring scope and in each capturing closure, so the index cannot group all
// their uses under one symbol; callers must not treat the index as complete for them.
func symbolIsFragmented(sym zeus_value.Value) bool {
	return zeus_value.AsRefCellVar(sym) != nil
}

// isValidIdentifier reports whether name is a legal Zeus identifier that is safe to rename to: a
// non-empty run of identifier characters not starting with a digit, and not a reserved keyword or
// built-in type name. Renaming to anything else would write source that no longer parses.
func isValidIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		isLetter := r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if isLetter || (i > 0 && isDigit) {
			continue
		}
		return false
	}
	if _, isKeyword := token.Keywords[name]; isKeyword {
		return false
	}
	if _, isType := token.DataTypes[name]; isType {
		return false
	}
	return true
}

// sortSpans orders spans by their start position, so highlight/reference results are deterministic.
func sortSpans(spans []*token.Span) {
	sort.Slice(spans, func(i, j int) bool {
		a, b := spans[i].Start, spans[j].Start
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

// getDocumentHighlights highlights every occurrence of the symbol under the cursor.
func (s *Server) getDocumentHighlights(uri protocol.DocumentURI, position protocol.Position) []protocol.DocumentHighlight {
	highlights := []protocol.DocumentHighlight{}
	docInfo, ok := s.doc(uri)
	if !ok {
		return highlights
	}
	for _, span := range s.occurrences(docInfo, position) {
		highlights = append(highlights, protocol.DocumentHighlight{Range: spanToRange(span)})
	}
	return highlights
}

// getReferences returns the locations of every occurrence of the symbol under the cursor. For a
// resolved symbol these are precise (via the binding index); for anything else it falls back to a
// single-file name-based scan.
func (s *Server) getReferences(uri protocol.DocumentURI, position protocol.Position) []protocol.Location {
	locations := []protocol.Location{}
	docInfo, ok := s.doc(uri)
	if !ok {
		return locations
	}
	for _, span := range s.occurrences(docInfo, position) {
		locations = append(locations, protocol.Location{URI: uri, Range: spanToRange(span)})
	}
	return locations
}

// renameTarget resolves the identifier under the cursor to a symbol that is safe to rename with
// single-file edits — a function-local variable or parameter (has a recorded def) that is not
// escape-boxed (whose occurrences the index cannot fully group). prepareRename and rename both go
// through it, so they can never disagree about what is renameable. It returns the declaration
// identifier's span (for the prepare highlight) and the symbol (for collecting occurrences).
func (s *Server) renameTarget(docInfo *DocumentInfo, position protocol.Position) (*token.Span, zeus_value.Value, bool) {
	if docInfo.AST == nil {
		return nil, nil, false
	}
	id, _ := identifierAt(docInfo.AST, zeusPos(position))
	if id == nil {
		return nil, nil, false
	}
	sym, ok := docInfo.Semantic.SymbolAt(id)
	if !ok || symbolIsFragmented(sym) {
		return nil, nil, false
	}
	if _, ok := docInfo.Semantic.DefNode(sym); !ok {
		return nil, nil, false
	}
	return id.GetSpan(), sym, true
}

// prepareRename reports whether the symbol under the cursor can be renamed, returning the range of
// the identifier. Only function-local variables and parameters are renameable: all of their
// references live in this file, so the edit is complete. Everything else (functions, classes,
// module-level and imported symbols, escape-boxed variables, keywords) returns nil, which the
// editor shows as "cannot rename here" — safer than an unsafe partial rename.
func (s *Server) prepareRename(uri protocol.DocumentURI, position protocol.Position) *protocol.Range {
	docInfo, ok := s.doc(uri)
	if !ok {
		return nil
	}
	declSpan, _, ok := s.renameTarget(docInfo, position)
	if !ok {
		return nil
	}
	r := spanToRange(declSpan)
	return &r
}

// rename produces the edits that rename the local variable or parameter under the cursor to
// newName. It shares renameTarget with prepareRename, so a rename is either complete or refused —
// never a corrupting partial edit — and rewrites every occurrence (declaration and references) of
// that exact binding.
func (s *Server) rename(uri protocol.DocumentURI, position protocol.Position, newName string) *protocol.WorkspaceEdit {
	// Reject an illegal target name up front so a rename never writes source that fails to parse.
	if !isValidIdentifier(newName) {
		return nil
	}
	docInfo, ok := s.doc(uri)
	if !ok {
		return nil
	}
	_, sym, ok := s.renameTarget(docInfo, position)
	if !ok {
		return nil
	}
	nodes := docInfo.Semantic.SymbolUses(sym)
	if len(nodes) == 0 {
		return nil
	}
	edits := make([]protocol.TextEdit, 0, len(nodes))
	for _, n := range nodes {
		edits = append(edits, protocol.TextEdit{Range: spanToRange(n.GetSpan()), NewText: newName})
	}
	return &protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{uri: edits},
	}
}

// signatureContext scans backward from the cursor (over lexed tokens, so strings/comments do
// not confuse bracket matching) to find the call the cursor is inside. It returns the callee
// identifier chain (e.g. ["obj","method"] or ["fn"]) and the zero-based index of the argument
// being typed. ok is false when the cursor is not inside a call's argument list.
func signatureContext(content string, line, col int) (chain []string, activeParam int, ok bool) {
	tokens, _ := lexer.NewLexer(content).Lex()

	// Index of the last token that starts strictly before the cursor.
	curLine, curCol := line+1, col+1 // spans are 1-based
	idx := -1
	for i, t := range tokens {
		if t.Type == token.TokenTypeEOF {
			break
		}
		start := t.Span.Start
		if start.Line < curLine || (start.Line == curLine && start.Column < curCol) {
			idx = i
			continue
		}
		break
	}
	if idx < 0 {
		return nil, 0, false
	}

	depth := 0
	parenIdx := -1
	for i := idx; i >= 0; i-- {
		switch tokens[i].Type {
		case token.TokenTypeRightParen, token.TokenTypeRightBracket, token.TokenTypeRightBrace:
			depth++
		case token.TokenTypeLeftBracket, token.TokenTypeLeftBrace:
			if depth == 0 {
				return nil, 0, false // inside an index/block, not a call
			}
			depth--
		case token.TokenTypeLeftParen:
			if depth == 0 {
				parenIdx = i
			} else {
				depth--
			}
		case token.TokenTypeComma:
			if depth == 0 {
				activeParam++
			}
		case token.TokenTypeSemicolon:
			if depth == 0 {
				return nil, 0, false
			}
		}
		if parenIdx >= 0 {
			break
		}
	}
	if parenIdx <= 0 {
		return nil, 0, false
	}

	// Read the callee chain backward: identifier ('.' identifier)*.
	var reversed []string
	i := parenIdx - 1
	for i >= 0 && tokens[i].Type == token.TokenTypeIdentifier {
		reversed = append(reversed, tokens[i].Value)
		if i-1 >= 0 && tokens[i-1].Type == token.TokenTypeDot {
			i -= 2
			continue
		}
		break
	}
	if len(reversed) == 0 {
		return nil, 0, false
	}
	for j := len(reversed) - 1; j >= 0; j-- {
		chain = append(chain, reversed[j])
	}
	return chain, activeParam, true
}

// resolveCallable maps a callee chain to the function whose signature should be shown: a
// free function, a class constructor (`new ClassName(`), or a method on a receiver.
func resolveCallable(irModule *ir.IRModule, chain []string) *zeus_value.Function {
	if len(chain) == 0 {
		return nil
	}
	if len(chain) == 1 {
		sym := symbolByName(irModule, chain[0])
		if fn := zeus_value.AsFunction(sym); fn != nil {
			return fn
		}
		if class := zeus_value.AsClass(sym); class != nil {
			if ctor := zeus_value.LookupMethod(class, token.CONSTRUCTOR_METHOD_NAME); ctor != nil {
				return ctor.Method
			}
		}
		return nil
	}
	r, ok := resolveReceiverChain(irModule, chain[:len(chain)-1])
	if !ok {
		return nil
	}
	if m, ok := lookupMember(r, chain[len(chain)-1]); ok && m.kind == memberMethod {
		return m.fn
	}
	return nil
}

// getSignatureHelp shows the parameter list of the call the cursor is inside, with the active
// parameter highlighted.
func (s *Server) getSignatureHelp(uri protocol.DocumentURI, position protocol.Position) *protocol.SignatureHelp {
	docInfo, ok := s.doc(uri)
	if !ok || docInfo.IRModule == nil {
		return nil
	}
	chain, activeParam, ok := signatureContext(docInfo.Content, int(position.Line), int(position.Character))
	if !ok {
		return nil
	}
	fn := resolveCallable(docInfo.IRModule, chain)
	if fn == nil {
		return nil
	}

	params := make([]protocol.ParameterInformation, 0, len(fn.Params))
	for _, p := range fn.Params {
		label := p.Name
		if p.ValueType != nil {
			label += ": " + p.ValueType.String()
		}
		params = append(params, protocol.ParameterInformation{Label: label})
	}

	return &protocol.SignatureHelp{
		Signatures: []protocol.SignatureInformation{{
			Label:      fn.SourceName() + funcSignature(fn),
			Parameters: params,
		}},
		ActiveSignature: 0,
		ActiveParameter: uint32(activeParam),
	}
}

// inlayHint mirrors the LSP 3.17 InlayHint shape. The protocol library in use predates inlay
// hints, so the type is declared locally and marshaled to the standard JSON.
type inlayHint struct {
	Position    protocol.Position `json:"position"`
	Label       string            `json:"label"`
	Kind        int               `json:"kind"` // 1 = Type, 2 = Parameter
	PaddingLeft bool              `json:"paddingLeft,omitempty"`
}

// inlayHintParams mirrors the LSP InlayHintParams (the request carries the visible range).
type inlayHintParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Range        protocol.Range                  `json:"range"`
}

const inlayHintKindType = 1

// getInlayHints shows the inferred type after `let`/`const` declarations that have no explicit
// annotation, using the type the type checker resolved for the variable.
func (s *Server) getInlayHints(params inlayHintParams) []inlayHint {
	hints := []inlayHint{}
	docInfo, ok := s.doc(params.TextDocument.URI)
	if !ok || docInfo.IRModule == nil {
		return hints
	}
	tokens, _ := lexer.NewLexer(docInfo.Content).Lex()

	for i, tok := range tokens {
		if tok.Type != token.TokenTypeLet && tok.Type != token.TokenTypeConst {
			continue
		}
		// Need `let/const IDENT` with no following `:` (i.e. no explicit annotation).
		if i+1 >= len(tokens) || tokens[i+1].Type != token.TokenTypeIdentifier {
			continue
		}
		nameTok := tokens[i+1]
		if i+2 < len(tokens) && tokens[i+2].Type == token.TokenTypeColon {
			continue
		}

		v := zeus_value.AsVar(symbolByName(docInfo.IRModule, nameTok.Value))
		if v == nil || v.ValueType == nil || zeus_value.IsUndefinedType(v.ValueType) {
			continue
		}
		// Scope safety: the symbol table holds one variable per name, so only emit the hint
		// when that variable was declared on this line (avoids mislabeling a same-named
		// variable from another scope).
		if v.Span == nil || v.Span.Start.Line != nameTok.Span.Start.Line {
			continue
		}

		pos := spanToRange(nameTok.Span).End
		if pos.Line < params.Range.Start.Line || pos.Line > params.Range.End.Line {
			continue
		}
		hints = append(hints, inlayHint{
			Position: pos,
			Label:    ": " + v.ValueType.String(),
			Kind:     inlayHintKindType,
		})
	}
	return hints
}
