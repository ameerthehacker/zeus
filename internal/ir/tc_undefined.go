package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// UndefinedTypeCheckPass ensures no variable reaches codegen with an unresolved type.
// It runs last, after TypeCheckingPass has had a chance to infer types from initializers.
type UndefinedTypeCheckPass struct{}

func NewUndefinedTypeCheckPass() *UndefinedTypeCheckPass {
	return &UndefinedTypeCheckPass{}
}

func (p *UndefinedTypeCheckPass) GetName() string {
	return "UndefinedTypeCheckPass"
}

func (p *UndefinedTypeCheckPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	if instr.Type != InstrTypeDeclVar {
		return
	}
	varDecl := AsDeclVarInstrInput(instr.Input)
	if zeus_value.IsUndefinedType(varDecl.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot infer type of '%s'", varDecl.Variable.Name),
			Span:    varDecl.Variable.Span,
		})
	}
}

func (p *UndefinedTypeCheckPass) Finalize(tc *TypeChecker) {}
