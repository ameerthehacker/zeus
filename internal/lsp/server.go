package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/lexer"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/parser"
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
	"go.lsp.dev/protocol"
)

type DocumentInfo struct {
	Content  string
	IRModule *ir.IRModule
	Errors   []*zeus_error.ZeusError
}

type Server struct {
	client    protocol.Client
	documents map[string]*DocumentInfo // URI -> DocumentInfo
}

func NewServer() *Server {
	return &Server{
		documents: make(map[string]*DocumentInfo),
	}
}

// jsonrpcMessage represents a JSON-RPC 2.0 message
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

// Initialize handles the initialize request
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: protocol.TextDocumentSyncOptions{
				OpenClose: true,
				Change:    protocol.TextDocumentSyncKindFull,
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{"."},
			},
			HoverProvider:          true,
			DefinitionProvider:     true,
			DocumentSymbolProvider: true,
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    "zeus-lsp",
			Version: "0.0.1",
		},
	}, nil
}

// Start starts the LSP server
func (s *Server) Start() error {
	logger.Log(zeus_error.ErrorSeverityInfo, "Zeus LSP server running")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(s.splitFunc)

	for scanner.Scan() {
		msg := scanner.Bytes()
		if err := s.handleMessage(msg); err != nil {
			fmt.Fprintf(os.Stderr, "Error handling message: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// splitFunc is a custom split function for reading LSP messages
func (s *Server) splitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for Content-Length header
	headerEnd := strings.Index(string(data), "\r\n\r\n")
	if headerEnd == -1 {
		if atEOF {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil // Request more data
	}

	headers := string(data[:headerEnd])
	contentLength := 0
	for _, line := range strings.Split(headers, "\r\n") {
		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(strings.TrimSpace(lengthStr))
			if err != nil {
				return 0, nil, err
			}
			break
		}
	}

	if contentLength == 0 {
		return 0, nil, fmt.Errorf("no Content-Length header")
	}

	totalLength := headerEnd + 4 + contentLength
	if len(data) < totalLength {
		if atEOF {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return 0, nil, nil // Request more data
	}

	return totalLength, data[headerEnd+4 : totalLength], nil
}

// handleMessage processes an LSP message
func (s *Server) handleMessage(msg []byte) error {
	var message jsonrpcMessage
	if err := json.Unmarshal(msg, &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Received message: %s\n", message.Method)

	var result interface{}
	var err error

	switch message.Method {
	case "initialize":
		var params protocol.InitializeParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result, err = s.Initialize(context.Background(), &params)
	case "initialized":
		// No response needed
		return nil
	case "shutdown":
		result = nil
		err = nil
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		var params protocol.DidOpenTextDocumentParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Document opened: %s\n", params.TextDocument.URI)
		// Validate and send diagnostics (this will store document info)
		return s.validateDocument(params.TextDocument.URI, params.TextDocument.Text)
	case "textDocument/didChange":
		var params protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Document changed: %s\n", params.TextDocument.URI)
		// Update document content (using full sync)
		if len(params.ContentChanges) > 0 {
			// Since we're using TextDocumentSyncKindFull, contentChanges will have the full text
			// Extract text from the first change event
			var changeEvent struct {
				Text string `json:"text"`
			}
			changeData, err := json.Marshal(params.ContentChanges[0])
			if err != nil {
				return err
			}
			if err := json.Unmarshal(changeData, &changeEvent); err != nil {
				return err
			}
			// Validate and send diagnostics (this will store document info)
			return s.validateDocument(params.TextDocument.URI, changeEvent.Text)
		}
		return nil
	case "textDocument/didClose":
		var params protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Document closed: %s\n", params.TextDocument.URI)
		// Remove document from cache
		delete(s.documents, string(params.TextDocument.URI))
		// Clear diagnostics
		return s.sendDiagnostics(params.TextDocument.URI, []protocol.Diagnostic{})
	case "textDocument/didSave":
		// Just acknowledge the save, no action needed
		return nil
	case "textDocument/hover":
		var params protocol.HoverParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = &protocol.Hover{
			Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: "Zeus language support",
			},
		}
	case "textDocument/completion":
		var params protocol.CompletionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		completions := s.getCompletions(params.TextDocument.URI, params.Position)
		result = completions
	case "textDocument/definition":
		result = []protocol.Location{}
	case "textDocument/documentSymbol":
		result = []protocol.DocumentSymbol{}
	default:
		// Unknown method
		return nil
	}

	if err != nil {
		return s.sendError(message.ID, err)
	}

	return s.sendResponse(message.ID, result)
}

// sendResponse sends a JSON-RPC response
func (s *Server) sendResponse(id interface{}, result interface{}) error {
	response := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.writeMessage(data)
}

// sendError sends a JSON-RPC error response
func (s *Server) sendError(id interface{}, err error) error {
	response := jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -32603,
			"message": err.Error(),
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	return s.writeMessage(data)
}

// writeMessage writes a message with LSP headers
func (s *Server) writeMessage(data []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := os.Stdout.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return err
	}
	return nil
}

// parseDocument parses a document and returns the IR module and any errors
// Returns partial results even when there are errors to support IDE features
func (s *Server) parseDocument(content string) (*ir.IRModule, []*zeus_error.ZeusError) {
	allErrors := []*zeus_error.ZeusError{}

	// Lexer phase
	lexer := lexer.NewLexer(content)
	tokens, lexerErrors := lexer.Lex()

	if len(lexerErrors) > 0 {
		allErrors = append(allErrors, lexerErrors...)
		// Continue even with lexer errors - we may have partial tokens
	}

	// If we have no tokens at all, we can't proceed
	if len(tokens) == 0 {
		return nil, allErrors
	}

	// Parser phase
	parser := parser.NewParser(tokens)
	program, parserErrors := parser.ParseProgram()

	if len(parserErrors) > 0 {
		allErrors = append(allErrors, parserErrors...)
		// Continue with partial AST - parser returns partial results
	}

	// If we have no program AST, we can't proceed
	if program == nil {
		return nil, allErrors
	}

	// IR Generation phase
	irBuilder := ir.NewIRBuilder()
	irModule := ir.NewIRModule(irBuilder, "lsp-document", func(modulePath string) *ir.IRModule {
		// For LSP, we don't resolve imports (yet)
		return nil
	})
	irErrors := irModule.Generate(program)

	if len(irErrors) > 0 {
		allErrors = append(allErrors, irErrors...)
		// Continue to type checking even if IR has some errors
	}

	// Type checking phase
	// Only run type checking if we have a valid IR builder
	if irBuilder != nil {
		typeChecker := ir.NewTypeChecker(irBuilder, true)
		typeErrors := typeChecker.TypeCheck()

		if len(typeErrors) > 0 {
			allErrors = append(allErrors, typeErrors...)
		}
	}

	return irModule, allErrors
}

// convertToLSPDiagnostics converts Zeus errors to LSP diagnostics
func (s *Server) convertToLSPDiagnostics(errors []*zeus_error.ZeusError) []protocol.Diagnostic {
	diagnostics := make([]protocol.Diagnostic, 0, len(errors))

	for _, err := range errors {
		var severity protocol.DiagnosticSeverity
		switch err.Severity {
		case zeus_error.ErrorSeverityError:
			severity = protocol.DiagnosticSeverityError
		case zeus_error.ErrorSeverityWarning:
			severity = protocol.DiagnosticSeverityWarning
		case zeus_error.ErrorSeverityInfo:
			severity = protocol.DiagnosticSeverityInformation
		default:
			severity = protocol.DiagnosticSeverityError
		}

		// Convert Zeus Span to LSP Range
		// Zeus uses 1-based line and column, LSP uses 0-based (so subtract 1)
		// Zeus uses INCLUSIVE end positions (End points to last char)
		// LSP uses EXCLUSIVE end positions (End points after last char)
		// So for End, we don't subtract 1, making it exclusive
		var startLine, startChar, endLine, endChar uint32
		if err.Span != nil {
			startLine = uint32(err.Span.Start.Line - 1)
			startChar = uint32(err.Span.Start.Column - 1)
			endLine = uint32(err.Span.End.Line - 1)
			endChar = uint32(err.Span.End.Column) // No -1: converts inclusive to exclusive
		}

		diagnostic := protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      startLine,
					Character: startChar,
				},
				End: protocol.Position{
					Line:      endLine,
					Character: endChar,
				},
			},
			Severity: severity,
			Source:   "zeus",
			Message:  err.Message,
		}

		diagnostics = append(diagnostics, diagnostic)
	}

	return diagnostics
}

// sendDiagnostics sends diagnostics notification to the client
func (s *Server) sendDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) error {
	notification := jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: json.RawMessage(func() []byte {
			data, _ := json.Marshal(protocol.PublishDiagnosticsParams{
				URI:         uri,
				Diagnostics: diagnostics,
			})
			return data
		}()),
	}

	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	return s.writeMessage(data)
}

// getKeywordDescription returns a human-readable description for a keyword
func getKeywordDescription(keyword string) string {
	descriptions := map[string]string{
		"let":       "Variable declaration",
		"const":     "Constant declaration",
		"function":  "Function declaration",
		"return":    "Return statement",
		"if":        "If statement",
		"else":      "Else statement",
		"while":     "While loop",
		"true":      "Boolean true",
		"false":     "Boolean false",
		"import":    "Import statement",
		"export":    "Export statement",
		"from":      "Import from",
		"class":     "Class declaration",
		"private":   "Private access modifier",
		"public":    "Public access modifier",
		"protected": "Protected access modifier",
		"new":       "Object instantiation",
		"null":      "Null value",
	}

	if desc, ok := descriptions[keyword]; ok {
		return desc
	}
	return fmt.Sprintf("Keyword: %s", keyword)
}

// getDataTypeDescription returns a human-readable description for a data type
func getDataTypeDescription(typeName string) string {
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
		"null":    "Null type",
	}

	if desc, ok := descriptions[typeName]; ok {
		return desc
	}
	return fmt.Sprintf("Type: %s", typeName)
}

// getCompletions returns completion items for the given document at a specific position
func (s *Server) getCompletions(uri protocol.DocumentURI, position protocol.Position) *protocol.CompletionList {
	items := []protocol.CompletionItem{}

	// Add Zeus keywords from the token package
	for keyword := range token.Keywords {
		items = append(items, protocol.CompletionItem{
			Label:  keyword,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: getKeywordDescription(keyword),
		})
	}

	// Add type keywords from the token package
	for dataType := range token.DataTypes {
		items = append(items, protocol.CompletionItem{
			Label:  dataType,
			Kind:   protocol.CompletionItemKindKeyword,
			Detail: getDataTypeDescription(dataType),
		})
	}

	// Get document-specific completions from symbol table
	docInfo, ok := s.documents[string(uri)]
	if ok && docInfo.IRModule != nil {
		symbolItems := s.getSymbolCompletions(docInfo.IRModule, position)
		items = append(items, symbolItems...)
	}

	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}

// getSymbolCompletions extracts completion items from the IR module's symbol table
// Only includes symbols declared before the given position
func (s *Server) getSymbolCompletions(irModule *ir.IRModule, cursorPosition protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	seen := make(map[string]bool)

	// Get all symbols from the IR module
	symbols := irModule.GetAllSymbols()

	// Convert cursor position to 1-based for comparison with Zeus spans
	cursorLine := int(cursorPosition.Line) + 1
	cursorColumn := int(cursorPosition.Character) + 1

	for name, value := range symbols {
		// Skip if we've already seen this symbol (duplicates in different scopes)
		if seen[name] {
			continue
		}

		// Skip temporary variables (they start with '%')
		if asVar := zeus_value.AsVar(value); asVar != nil && asVar.IsTempVariable() {
			continue
		}

		// Check if the symbol is declared before the cursor position
		var symbolSpan *token.Span
		if asVar := zeus_value.AsVar(value); asVar != nil {
			symbolSpan = asVar.Span
		} else if asFunc := zeus_value.AsFunction(value); asFunc != nil {
			symbolSpan = asFunc.Span
		} else if asClass := zeus_value.AsClass(value); asClass != nil {
			symbolSpan = asClass.Span
		}

		// Skip symbols declared after the cursor
		if symbolSpan != nil {
			// If the symbol starts after the cursor line, skip it
			if symbolSpan.Start.Line > cursorLine {
				continue
			}
			// If on the same line, check column position
			if symbolSpan.Start.Line == cursorLine && symbolSpan.Start.Column > cursorColumn {
				continue
			}
		}

		seen[name] = true

		// Determine the completion item kind and detail based on the symbol type
		var kind protocol.CompletionItemKind
		var detail string
		var documentation string

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
			// Build function signature
			params := []string{}
			for _, param := range asFunc.Params {
				if param.ValueType != nil {
					params = append(params, fmt.Sprintf("%s: %s", param.Name, param.ValueType.String()))
				} else {
					params = append(params, param.Name)
				}
			}
			returnType := "void"
			if asFunc.ReturnType != nil {
				returnType = asFunc.ReturnType.String()
			}
			detail = fmt.Sprintf("(%s): %s", strings.Join(params, ", "), returnType)
			documentation = fmt.Sprintf("Function %s", detail)
		} else if asClass := zeus_value.AsClass(value); asClass != nil {
			kind = protocol.CompletionItemKindClass
			detail = "class"
			documentation = fmt.Sprintf("Class %s", asClass.Name)
		} else {
			// Unknown symbol type, skip it
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

// validateDocument parses a document and sends diagnostics
func (s *Server) validateDocument(uri protocol.DocumentURI, content string) error {
	irModule, errors := s.parseDocument(content)

	// Store the document info
	s.documents[string(uri)] = &DocumentInfo{
		Content:  content,
		IRModule: irModule,
		Errors:   errors,
	}

	diagnostics := s.convertToLSPDiagnostics(errors)

	// Log diagnostics for debugging
	fmt.Fprintf(os.Stderr, "Generated %d diagnostics for %s\n", len(diagnostics), uri)

	return s.sendDiagnostics(uri, diagnostics)
}
