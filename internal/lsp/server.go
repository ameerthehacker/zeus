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

	"go.lsp.dev/protocol"
)

type Server struct {
	client protocol.Client
}

func NewServer() *Server {
	return &Server{}
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
		return nil
	case "textDocument/didChange":
		var params protocol.DidChangeTextDocumentParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Document changed: %s\n", params.TextDocument.URI)
		return nil
	case "textDocument/didClose":
		var params protocol.DidCloseTextDocumentParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Document closed: %s\n", params.TextDocument.URI)
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
		result = &protocol.CompletionList{
			IsIncomplete: false,
			Items: []protocol.CompletionItem{
				{Label: "let", Kind: protocol.CompletionItemKindKeyword, Detail: "Variable declaration"},
				{Label: "fn", Kind: protocol.CompletionItemKindKeyword, Detail: "Function declaration"},
				{Label: "class", Kind: protocol.CompletionItemKindKeyword, Detail: "Class declaration"},
				{Label: "if", Kind: protocol.CompletionItemKindKeyword, Detail: "If statement"},
				{Label: "while", Kind: protocol.CompletionItemKindKeyword, Detail: "While loop"},
				{Label: "return", Kind: protocol.CompletionItemKindKeyword, Detail: "Return statement"},
			},
		}
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
