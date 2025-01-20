package error

import "ameerthehacker/zeus/internal/common/token"

type ErrorSeverity string

const (
	ErrorSeverityError ErrorSeverity = "error"
	ErrorSeverityWarning ErrorSeverity = "warning"
)

type ZeusError struct {
	Message string
	Severity ErrorSeverity
	Span token.Span
}

func NewZeusError(severity ErrorSeverity, message string, span token.Span) ZeusError {
	return ZeusError{Severity: severity, Message: message, Span: span}
}
