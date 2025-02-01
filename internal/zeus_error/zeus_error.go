package zeus_error

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/token"
)

type ErrorSeverity int

const (
	ErrorSeverityError ErrorSeverity = iota
	ErrorSeverityWarning
	ErrorSeverityInfo
)

func (e ErrorSeverity) String() string {
	switch e {
	case ErrorSeverityError:
		return "error"
	case ErrorSeverityWarning:
		return "warning"
	case ErrorSeverityInfo:
		return "info"
	default:
		panic("unknown error severity")
	}
}

type ZeusError struct {
	Message string
	Severity ErrorSeverity
	Span *token.Span
}

func NewZeusError(severity ErrorSeverity, message string, span *token.Span) *ZeusError {
	return &ZeusError{Severity: severity, Message: message, Span: span}
}

func (e *ZeusError) Equal(other *ZeusError) bool {
	return e.Severity == other.Severity && e.Message == other.Message && e.Span.Start == other.Span.Start && e.Span.End == other.Span.End
}

func (e *ZeusError) String() string {
	return fmt.Sprintf("{Severity: %s, Message: %s, Span: %s}", e.Severity, e.Message, e.Span)
}

func Assert(condition bool, message string) {
	if !condition {
		panic(message)
	}
}
