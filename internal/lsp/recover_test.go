package lsp

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
// The LSP writes protocol messages straight to os.Stdout, so this is how we observe them.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	fn()

	_ = w.Close()
	os.Stdout = orig
	return <-done
}

// TestReportInternalErrorNotifiesOnce verifies that a recovered internal error is surfaced to
// the editor via window/showMessage, and that identical failures are only shown once (so
// rapid typing does not spam popups).
func TestReportInternalErrorNotifiesOnce(t *testing.T) {
	s := NewServer()
	stack := []byte("goroutine 1 [running]:\ngithub.com/ameerthehacker/zeus/internal/ir.(*IRModule).Boom()\n\t/x/internal/ir/ir.go:42 +0x1\n")

	out := captureStdout(t, func() {
		s.reportInternalError("textDocument/didChange", "boom", stack)
	})

	if !strings.Contains(out, "window/showMessage") {
		t.Errorf("expected a window/showMessage notification, got: %q", out)
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("notification should include the panic value, got: %q", out)
	}
	if !strings.Contains(out, "internal/ir") {
		t.Errorf("notification should point at the failing frame, got: %q", out)
	}
	if !strings.Contains(out, "report") {
		t.Errorf("notification should ask the user to report it, got: %q", out)
	}

	// A second identical failure must be suppressed.
	out2 := captureStdout(t, func() {
		s.reportInternalError("textDocument/didChange", "boom", stack)
	})
	if strings.Contains(out2, "window/showMessage") {
		t.Errorf("duplicate internal error should not be shown again, got: %q", out2)
	}

	// A different failure is still reported.
	out3 := captureStdout(t, func() {
		s.reportInternalError("textDocument/hover", "kaboom", stack)
	})
	if !strings.Contains(out3, "window/showMessage") {
		t.Errorf("a distinct internal error should be reported, got: %q", out3)
	}
}

// TestUnsupportedRequestReturnsMethodNotFound verifies that a request for a method the server
// does not implement gets a JSON-RPC "method not found" error, so the client does not hang.
func TestUnsupportedRequestReturnsMethodNotFound(t *testing.T) {
	s := NewServer()
	body := []byte(`{"jsonrpc":"2.0","id":7,"method":"textDocument/codeAction","params":{}}`)
	out := captureStdout(t, func() { _ = s.handleMessage(body) })
	if !strings.Contains(out, "-32601") {
		t.Errorf("unsupported request should return method-not-found (-32601), got: %q", out)
	}
	if !strings.Contains(out, `"id":7`) {
		t.Errorf("error response must echo the request id, got: %q", out)
	}
}

// TestUnknownNotificationIgnored verifies that an unknown notification (no id) produces no
// response — notifications must never be answered.
func TestUnknownNotificationIgnored(t *testing.T) {
	s := NewServer()
	body := []byte(`{"jsonrpc":"2.0","method":"$/somethingCustom","params":{}}`)
	out := captureStdout(t, func() { _ = s.handleMessage(body) })
	if strings.TrimSpace(out) != "" {
		t.Errorf("unknown notification should produce no response, got: %q", out)
	}
}

// TestFirstZeusFrame checks the stack-frame extraction used in the report message.
func TestFirstZeusFrame(t *testing.T) {
	stack := []byte("panic: boom\ngithub.com/ameerthehacker/zeus/internal/parser.(*Parser).consume()\n\t/any/path/parser.go:1\n")
	got := firstZeusFrame(stack)
	if !strings.Contains(got, "internal/parser") {
		t.Errorf("firstZeusFrame = %q, want it to name internal/parser", got)
	}
	if firstZeusFrame([]byte("no zeus frames here")) != "" {
		t.Errorf("firstZeusFrame should return empty when no zeus frame is present")
	}
}
