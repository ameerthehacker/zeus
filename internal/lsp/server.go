package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/logger"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"go.lsp.dev/protocol"
)

// This file holds the server type and the request dispatcher. The wire protocol lives in
// transport.go, the compile pipeline in pipeline.go, diagnostics in diagnostics.go, and each
// language feature in its own file (completion.go, hover.go, symbols.go, navigation.go,
// imports.go), with shared resolution helpers in analysis.go.

type DocumentInfo struct {
	Content  string
	IRModule *ir.IRModule
	Errors   []*zeus_error.ZeusError
}

type Server struct {
	client    protocol.Client
	documents map[string]*DocumentInfo // URI -> DocumentInfo
	// reportedPanics remembers panic signatures already surfaced to the user so the same
	// internal error is not popped up on every keystroke (didChange fires constantly).
	reportedPanics map[string]bool
}

func NewServer() *Server {
	return &Server{
		documents:      make(map[string]*DocumentInfo),
		reportedPanics: make(map[string]bool),
	}
}

// doc returns the cached document for a URI, or ok=false if none is open.
func (s *Server) doc(uri protocol.DocumentURI) (*DocumentInfo, bool) {
	d, ok := s.documents[string(uri)]
	return d, ok
}

// Initialize handles the initialize request. Capabilities are built as a map so the server can
// advertise features (e.g. inlayHintProvider) that predate the protocol library's struct.
func (s *Server) Initialize(ctx context.Context, params *protocol.InitializeParams) (interface{}, error) {
	return map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    protocol.TextDocumentSyncKindFull,
			},
			"completionProvider": map[string]interface{}{
				// "." for members; '"' and "/" auto-trigger import path/symbol completion.
				"triggerCharacters": []string{".", "\"", "/"},
			},
			"signatureHelpProvider": map[string]interface{}{
				"triggerCharacters": []string{"(", ","},
			},
			"hoverProvider":              true,
			"definitionProvider":         true,
			"documentSymbolProvider":     true,
			"documentHighlightProvider":  true,
			"referencesProvider":         true,
			"inlayHintProvider":          true,
			"documentFormattingProvider": true,
		},
		"serverInfo": map[string]interface{}{
			"name":    "zeus-lsp",
			"version": "0.0.1",
		},
	}, nil
}

// Start runs the read loop, dispatching each framed message until stdin closes.
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

// handleMessage decodes and dispatches a single LSP message.
func (s *Server) handleMessage(msg []byte) (err error) {
	var message jsonrpcMessage
	if err := json.Unmarshal(msg, &message); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// A language server must outlive any single request: an editor sends a constant stream
	// of half-typed, syntactically broken documents. Recover from any panic a handler
	// raises, log it for diagnosis, and — for requests — reply with a JSON-RPC error so the
	// client is not left waiting. This is the last line of defence behind the root-cause
	// fixes in the lexer/parser/IR pipeline.
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fmt.Fprintf(os.Stderr, "recovered from panic while handling %q: %v\n%s\n", message.Method, r, stack)
			// Surface the failure so it is not silently swallowed — the user can then file
			// a bug report. Deduplicated so rapid typing does not spam identical popups.
			s.reportInternalError(message.Method, r, stack)
			if message.ID != nil {
				err = s.sendError(message.ID, fmt.Errorf("internal error handling %s", message.Method))
			} else {
				err = nil
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "Received message: %s\n", message.Method)

	var result interface{}

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
		// getHover returns nil when there is nothing to show; the JSON-RPC null result is a
		// valid "no hover" response.
		result = s.getHover(params.TextDocument.URI, params.Position)
	case "textDocument/completion":
		var params protocol.CompletionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getCompletions(params.TextDocument.URI, params.Position)
	case "textDocument/definition":
		var params protocol.DefinitionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getDefinition(params.TextDocument.URI, params.Position)
	case "textDocument/documentSymbol":
		var params protocol.DocumentSymbolParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getDocumentSymbols(params.TextDocument.URI)
	case "textDocument/signatureHelp":
		var params protocol.SignatureHelpParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getSignatureHelp(params.TextDocument.URI, params.Position)
	case "textDocument/documentHighlight":
		var params protocol.DocumentHighlightParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getDocumentHighlights(params.TextDocument.URI, params.Position)
	case "textDocument/references":
		var params protocol.ReferenceParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getReferences(params.TextDocument.URI, params.Position)
	case "textDocument/inlayHint":
		var params inlayHintParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.getInlayHints(params)
	case "textDocument/formatting":
		var params protocol.DocumentFormattingParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		result = s.formatDocument(params.TextDocument.URI)
	default:
		// Unknown/unsupported method. A request (has an id) must still get a reply, or the
		// client blocks waiting for one — respond with JSON-RPC "method not found". A
		// notification (no id) expects no response, so ignore it.
		if message.ID != nil {
			return s.sendMethodNotFound(message.ID, message.Method)
		}
		return nil
	}

	if err != nil {
		return s.sendError(message.ID, err)
	}

	return s.sendResponse(message.ID, result)
}
