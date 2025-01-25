package error

import (
	"ameerthehacker/zeus/internal/token"
	"fmt"
)

type ErrorSeverity string

const (
	ErrorSeverityError ErrorSeverity = "error"
	ErrorSeverityWarning ErrorSeverity = "warning"
)

type ZeusError struct {
	Message string
	Severity ErrorSeverity
	Span *token.Span
}

func NewZeusError(severity ErrorSeverity, message string, span *token.Span) *ZeusError {
	return &ZeusError{Severity: severity, Message: message, Span: span}
}

func (e *ZeusError) IsEqual(other *ZeusError) bool {
	return e.Severity == other.Severity && e.Message == other.Message && e.Span.Start == other.Span.Start && e.Span.End == other.Span.End
}

func (e *ZeusError) String() string {
	return fmt.Sprintf("{Severity: %s, Message: %s, Span: %s}", e.Severity, e.Message, e.Span)
}
