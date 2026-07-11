package lsp

import (
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

// getDocumentHighlights highlights every occurrence of the identifier under the cursor.
func (s *Server) getDocumentHighlights(uri protocol.DocumentURI, position protocol.Position) []protocol.DocumentHighlight {
	highlights := []protocol.DocumentHighlight{}
	docInfo, ok := s.doc(uri)
	if !ok {
		return highlights
	}
	word, _ := wordAt(docInfo.Content, int(position.Line), int(position.Character))
	if word == "" {
		return highlights
	}
	for _, span := range identifierOccurrences(docInfo.Content, word) {
		highlights = append(highlights, protocol.DocumentHighlight{Range: spanToRange(span)})
	}
	return highlights
}

// getReferences returns the locations of every occurrence of the identifier under the cursor.
// This is single-file only; cross-file references require module resolution the LSP does not
// yet do.
func (s *Server) getReferences(uri protocol.DocumentURI, position protocol.Position) []protocol.Location {
	locations := []protocol.Location{}
	docInfo, ok := s.doc(uri)
	if !ok {
		return locations
	}
	word, _ := wordAt(docInfo.Content, int(position.Line), int(position.Character))
	if word == "" {
		return locations
	}
	for _, span := range identifierOccurrences(docInfo.Content, word) {
		locations = append(locations, protocol.Location{URI: uri, Range: spanToRange(span)})
	}
	return locations
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
