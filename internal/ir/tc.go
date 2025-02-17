package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/symbol_table"
	"github.com/ameerthehacker/zeus/internal/value"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
)

type TypeChecker struct {
	symbol_table *symbol_table.SymbolTable[value.Value]
	errors []*zeus_error.ZeusError
}

func NewTypeChecker() *TypeChecker {
	return &TypeChecker{
		symbol_table: symbol_table.NewSymbolTable[value.Value](),
	}
}

func (tc *TypeChecker) pushError(err *zeus_error.ZeusError) {
	tc.errors = append(tc.errors, err)
}

func (tc *TypeChecker) tcFuncDecl(instr *Instr) {
	tc.symbol_table.EnterScope()
	func_decl := AsDeclFuncInstrInput(instr.Input)
	params := []*value.Var{}
	for _, param := range func_decl.Args {
		tc.symbol_table.DeclareSymbol(param.Name, &value.Var{
			Name: param.Name,
			ValueType: param.ValueType,
			Span: param.Span,
		})
		params = append(params, &value.Var{
			Name: param.Name,
			ValueType: param.ValueType,
			Span: param.Span,
		})
	}

	tc.symbol_table.DeclareGlobalSymbol(func_decl.Name, &value.Function{
		Params: params,
		ReturnType: func_decl.ReturnType,
		Span: instr.Span,
		Name: func_decl.Name,
	})

	tc.symbol_table.ExitScope()
}

func (tc *TypeChecker) tcDeclVar(instr *Instr) {
	decl_var := AsDeclVarInstrInput(instr.Input)
	tc.symbol_table.DeclareSymbol(decl_var.Variable.Name, decl_var.Variable)

	if decl_var.Initializer != nil {
		if !tc.cmpValueType(decl_var.Variable.ValueType, tc.getValueType(decl_var.Initializer)) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", decl_var.Variable.ValueType, tc.getValueType(decl_var.Initializer)),
				Span: decl_var.Variable.Span,
			})
		}
	}
}

func (tc *TypeChecker) getValueType(_value value.Value) value.ValueType {
	switch _value := _value.(type) {
	case *value.Var:
		return _value.ValueType
	case *value.Function:
		return value.ToFunctionType(*_value)
	case *value.Constant:
		return _value.ValueType
	default:
		panic(fmt.Sprintf("unknown value: %T", _value))
	}
}

func (tc *TypeChecker) cmpValueType(a, b value.ValueType) bool {
	switch a := a.(type) {
	case *value.IntType:
		b, ok := b.(*value.IntType)
		
		if !ok {
			return false
		}

		return a.Size >= b.Size
	case *value.FloatType:
		// TODO: support int type also
		b, ok := b.(*value.FloatType)
		if !ok {
			return false
		}
		return a.Size >= b.Size
	case *value.BoolType:
		_, ok := b.(*value.BoolType)
		if !ok {
			return false
		}
		return true
	case *value.FunctionType:
		b, ok := b.(*value.FunctionType)
		if !ok || len(a.ParamTypes) != len(b.ParamTypes) {
			return false
		}

		isReturnTypeEqual := tc.cmpValueType(a.ReturnType, b.ReturnType)
		
		if !isReturnTypeEqual {
			return false
		}

		for i := range a.ParamTypes {
			if !tc.cmpValueType(a.ParamTypes[i], b.ParamTypes[i]) {
				return false
			}
		}

		return true
	}

	return false
}

func (tc *TypeChecker) TypeCheck(instrs []*Instr) []*zeus_error.ZeusError {
	worklist := []*BasicBlock{}

	tc.symbol_table.EnterScope()

	for _, instr := range instrs {
		switch instr.Type {
		case InstrTypeDeclFunc:
			tc.tcFuncDecl(instr)
			worklist = append(worklist, AsDeclFuncInstrInput(instr.Input).Body)
		case InstrTypeDeclVar:
			tc.tcDeclVar(instr)
		default:
			panic(fmt.Sprintf("unhandled instruction type: %s", instr.Type))
		}

		for len(worklist) > 0 {
			block := worklist[0]
			tc.TypeCheck(block.Instrs)
			worklist = worklist[1:]
			worklist = append(worklist, block.Successors...)
		}
	}

	tc.symbol_table.ExitScope()

	return tc.errors
}
