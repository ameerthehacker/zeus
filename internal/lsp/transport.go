package lsp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"go.lsp.dev/protocol"
)

// This file implements the JSON-RPC / LSP wire protocol: reading Content-Length framed
// messages off stdin and writing framed responses and notifications back to stdout.

// jsonrpcMessage represents a JSON-RPC 2.0 message.
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

// splitFunc is a bufio.SplitFunc that yields one LSP message body per Content-Length frame.
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

// writeMessage writes a message with LSP Content-Length headers.
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

// sendResponse sends a JSON-RPC response.
func (s *Server) sendResponse(id interface{}, result interface{}) error {
	data, err := json.Marshal(jsonrpcMessage{JSONRPC: "2.0", ID: id, Result: result})
	if err != nil {
		return err
	}
	return s.writeMessage(data)
}

// sendError sends a JSON-RPC internal-error (-32603) response.
func (s *Server) sendError(id interface{}, err error) error {
	data, marshalErr := json.Marshal(jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -32603,
			"message": err.Error(),
		},
	})
	if marshalErr != nil {
		return marshalErr
	}
	return s.writeMessage(data)
}

// sendMethodNotFound replies to an unsupported request with the JSON-RPC "method not found"
// error (-32601) so the client stops waiting for a response.
func (s *Server) sendMethodNotFound(id interface{}, method string) error {
	data, err := json.Marshal(jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    -32601,
			"message": fmt.Sprintf("method not supported: %s", method),
		},
	})
	if err != nil {
		return err
	}
	return s.writeMessage(data)
}

// sendNotification marshals params and writes a JSON-RPC notification (a message with no id).
func (s *Server) sendNotification(method string, params interface{}) error {
	paramData, err := json.Marshal(params)
	if err != nil {
		return err
	}
	data, err := json.Marshal(jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  json.RawMessage(paramData),
	})
	if err != nil {
		return err
	}
	return s.writeMessage(data)
}

// sendDiagnostics sends a textDocument/publishDiagnostics notification to the client.
func (s *Server) sendDiagnostics(uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) error {
	return s.sendNotification("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}
