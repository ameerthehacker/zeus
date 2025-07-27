package ir

import (
	"fmt"
	"strings"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// TCPass represents a pluggable pass that can be run on the IR
// Passes receive individual instructions with automatic context management
type TCPass interface {
	// HandleInstruction processes a single instruction
	// The TypeChecker automatically manages currentFunction, currentClass, and currentBlock
	HandleInstruction(tc *TypeChecker, instr *Instr)
	
	// Finalize is called after all instructions have been processed
	// This allows passes to perform cleanup or final analysis
	Finalize(tc *TypeChecker)
	
	// GetName returns the name of the pass for debugging/logging
	GetName() string
}

type TypeChecker struct {
	errors          []*zeus_error.ZeusError
	currentFunction *zeus_value.Function
	currentClass *zeus_value.Class
	builder         *IRBuilder
	currentBlock    *BasicBlock
	passes          []TCPass
	IsEntryPoint    bool
}

func NewTypeChecker(builder *IRBuilder, isEntryPoint bool) *TypeChecker {
	tc := &TypeChecker{
		builder: builder,
		IsEntryPoint: isEntryPoint,
	}
	
	// Initialize required passes
	tc.passes = []TCPass{
		NewToKnownTypesPass(),
		NewIdentifierUsagePass(),
		NewTypeCheckingPass(),
		NewUnusedWarningPass(),
	}
	
	return tc
}

// AddPass adds a new pass to the type checker
func (tc *TypeChecker) AddPass(pass TCPass) {
	tc.passes = append(tc.passes, pass)
}

// TypeCheck runs all registered passes and returns accumulated errors
func (tc *TypeChecker) TypeCheck() []*zeus_error.ZeusError {
	tc.errors = nil // Reset errors
	
	for _, pass := range tc.passes {
		tc.runPass(pass)
	}
	
	return tc.errors
}

// runPass executes a single pass with automatic context management
func (tc *TypeChecker) runPass(pass TCPass) {
	tc.builder.Walk(func(instr *Instr) {
		// Automatically update context based on instruction type
		tc.updateContext(instr)
		
		// Check if instruction is allowed in current context
		if !tc.isInstructionAllowed(instr) {
			tc.pushError(&zeus_error.ZeusError{
				Message: "statement not supported outside of function",
				Span:    instr.Span,
			})
			return
		}
		
		// Let the pass handle the instruction
		pass.HandleInstruction(tc, instr)
	}, func(block *BasicBlock) {
		tc.currentBlock = block
	})
	
	// Call finalize after processing all instructions
	pass.Finalize(tc)
}

// updateContext automatically updates the current function and class context
func (tc *TypeChecker) updateContext(instr *Instr) {
	switch instr.Type {
	case InstrTypeDeclFunc:
		input := AsDeclFuncInstrInput(instr.Input)
		tc.currentFunction = input.Function
	case InstrTypeDeclClassMethod:
		input := AsDeclClassMethodInstrInput(instr.Input)
		tc.currentClass = input.Class
		tc.currentFunction = input.Method
	case InstrTypeDeclClass:
		input := AsDeclClassInstrInput(instr.Input)
		tc.currentClass = input.Class
	}
}

// isInstructionAllowed checks if an instruction is allowed in the current context
func (tc *TypeChecker) isInstructionAllowed(instr *Instr) bool {
	isTopLevelInstr := IsFunctionDeclInstr(instr.Type) || IsClassDeclInstr(instr.Type) || IsClassMethodDeclInstr(instr.Type) || IsExportInstr(instr.Type) || IsImportInstr(instr.Type)
	return isTopLevelInstr || tc.currentFunction != nil
}

// Common utility functions shared by all passes

// getBuiltInValueType returns the built-in value type of a value
func (tc *TypeChecker) getBuiltInValueType(value zeus_value.Value) zeus_value.ValueType {
	switch value := value.(type) {
	case *zeus_value.Var:
		return value.ValueType
	case *zeus_value.Function:
		return zeus_value.ToFunctionType(*value)
	case *zeus_value.Constant:
		return value.ValueType
	case *zeus_value.Object:
		return value.ValueType
	case *zeus_value.Class:
		return zeus_value.NewObjectType(*value)
	default:
		return zeus_value.UndefinedType{}
	}
}

// getValueType returns the value type of a value
func (tc *TypeChecker) getValueType(value zeus_value.Value) zeus_value.ValueType {
	valueType := tc.getBuiltInValueType(value)

	if valueType == nil {
		return zeus_value.UndefinedType{}
	}

	return valueType
}

func (tc *TypeChecker) pushError(err *zeus_error.ZeusError) {
	tc.errors = append(tc.errors, err)
}

// ToKnownTypesPass converts all UserDefinedType references to their actual known types
type ToKnownTypesPass struct{}

func NewToKnownTypesPass() *ToKnownTypesPass {
	return &ToKnownTypesPass{}
}

func (p *ToKnownTypesPass) GetName() string {
	return "ToKnownTypesPass"
}

func (p *ToKnownTypesPass) Finalize(tc *TypeChecker) {
	// No finalization needed for this pass
}

func (p *ToKnownTypesPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	// Resolve output type if it exists and is a UserDefinedType
	if instr.Output != nil && instr.Output.ValueType != nil {
		instr.Output.ValueType = p.resolveValueType(tc, instr.Output.ValueType)
	}
	
	switch instr.Type {
	case InstrTypeDeclVar:
		p.resolveVarDecl(tc, instr)
	case InstrTypeDeclFunc:
		p.resolveFuncDecl(tc, instr)
	case InstrTypeDeclClass:
		p.resolveClassDecl(tc, instr)
	case InstrTypeDeclClassMethod:
		p.resolveClassMethodDecl(tc, instr)
	}
}

// resolveValueType converts a UserDefinedType to its actual known type
func (p *ToKnownTypesPass) resolveValueType(tc *TypeChecker, valueType zeus_value.ValueType) zeus_value.ValueType {
	if zeus_value.IsUserDefinedType(valueType) {
		userDefinedType := zeus_value.AsUserDefinedType(valueType)
		variable, ok := tc.builder.symbolTable.GetSymbol(userDefinedType.Name)

		if !ok {
			// Return the original type if we can't resolve it
			// The TypeCheckingPass will handle the error
			return valueType
		}

		return tc.getBuiltInValueType(variable)
	}

	return valueType
}

func (p *ToKnownTypesPass) resolveVarDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclVarInstrInput(instr.Input)
	input.Variable.ValueType = p.resolveValueType(tc, input.Variable.ValueType)
}

func (p *ToKnownTypesPass) resolveFuncDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclFuncInstrInput(instr.Input)
	
	// Resolve return type
	input.Function.ReturnType = p.resolveValueType(tc, input.Function.ReturnType)
	
	// Resolve parameter types
	for i := range input.Function.Params {
		input.Function.Params[i].ValueType = p.resolveValueType(tc, input.Function.Params[i].ValueType)
	}
}

func (p *ToKnownTypesPass) resolveClassDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassInstrInput(instr.Input)
	
	// Resolve property types
	for i := range input.Class.Properties {
		input.Class.Properties[i].Property.ValueType = p.resolveValueType(tc, input.Class.Properties[i].Property.ValueType)
	}
	
	// Resolve method types
	for i := range input.Class.Methods {
		method := input.Class.Methods[i].Method
		method.ReturnType = p.resolveValueType(tc, method.ReturnType)
		
		for j := range method.Params {
			method.Params[j].ValueType = p.resolveValueType(tc, method.Params[j].ValueType)
		}
	}
}

func (p *ToKnownTypesPass) resolveClassMethodDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassMethodInstrInput(instr.Input)
	
	// Resolve return type
	input.Method.ReturnType = p.resolveValueType(tc, input.Method.ReturnType)
	
	// Resolve parameter types
	for i := range input.Method.Params {
		input.Method.Params[i].ValueType = p.resolveValueType(tc, input.Method.Params[i].ValueType)
	}
}



// IdentifierUsagePass checks variable initialization status
// and ensures variables are initialized before use
type IdentifierUsagePass struct{}

func NewIdentifierUsagePass() *IdentifierUsagePass {
	return &IdentifierUsagePass{}
}

func (p *IdentifierUsagePass) GetName() string {
	return "VariableInitializationPass"
}

func (p *IdentifierUsagePass) Finalize(tc *TypeChecker) {
	// No finalization needed for this pass
}

func (p *IdentifierUsagePass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	switch instr.Type {
	case InstrTypeDeclVar:
		p.handleVarDecl(instr)
	case InstrTypeStore:
		p.handleStore(instr)
	case InstrTypeLoad:
		p.handleLoad(tc, instr)
	}
}

// handleVarDecl processes variable declarations and sets IsInitialized if there's an initializer
func (p *IdentifierUsagePass) handleVarDecl(instr *Instr) {
	input := AsDeclVarInstrInput(instr.Input)
	
	// If the variable has an initializer, mark it as initialized
	if input.Initializer != nil {
		input.Variable.IsInitialized = true
	}
}

// handleStore processes variable assignments and marks the target variable as initialized
func (p *IdentifierUsagePass) handleStore(instr *Instr) {
	input := AsStoreInstrInput(instr.Input)
	
	// Mark the target variable as initialized
	input.Addr.IsInitialized = true
}

// handleLoad processes variable usage and checks initialization status
func (p *IdentifierUsagePass) handleLoad(tc *TypeChecker, instr *Instr) {
	input := AsLoadInstrInput(instr.Input)
	
	// Check if the variable is initialized before use
	if !input.Addr.IsInitialized && !input.Addr.IsTempVariable() {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("identifier '%s' used before being initialized", input.Addr.Name),
			Span:    instr.Span,
		})
	}
}

// This pass does the actual type checking
type TypeCheckingPass struct{
	hasMainFunction bool
}

func NewTypeCheckingPass() *TypeCheckingPass {
	return &TypeCheckingPass{
		hasMainFunction: false,
	}
}

func (p *TypeCheckingPass) GetName() string {
	return "TypeCheckingPass"
}

func (p *TypeCheckingPass) Finalize(tc *TypeChecker) {
	if !p.hasMainFunction && tc.IsEntryPoint {
		tc.pushError(&zeus_error.ZeusError{
			Message: "main function not found",
			Span:   nil,
		})
	}
}

func (p *TypeCheckingPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	switch instr.Type {
	// jmp requires no type checking
	case InstrTypeJmp:
	case InstrTypeDeclFunc:
		p.tcFuncDecl(tc, instr)
	case InstrTypeDeclVar:
		p.tcDeclVar(tc, instr)
	case InstrTypeAdd:
		fallthrough
	case InstrTypeSub:
		fallthrough
	case InstrTypeMul:
		fallthrough
	case InstrTypeDiv:
		p.tcBinaryOp(tc, instr, zeus_value.GetBiggerType, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
		})
	case InstrTypeEqEq:
		fallthrough
	case InstrTypeNotEq:
		fallthrough
	case InstrTypeLessThan:
		fallthrough
	case InstrTypeGreaterThan:
		fallthrough
	case InstrTypeLessThanEq:
		fallthrough
	case InstrTypeGreaterThanEq:
		p.tcBinaryOp(tc, instr, func(_, _ zeus_value.ValueType) zeus_value.ValueType {
			return zeus_value.BoolType{}
		}, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
		})
	case InstrTypeNot:
		p.tcUnaryOp(tc, instr, func(_ zeus_value.ValueType) zeus_value.ValueType {
			return zeus_value.BoolType{}
		}, zeus_value.IsBoolType)
	case InstrTypeNeg:
		p.tcUnaryOp(tc, instr, func(operandType zeus_value.ValueType) zeus_value.ValueType {
			switch operandType := operandType.(type) {
			// negation on int makes it signed
			case zeus_value.IntType:
				return zeus_value.IntType{
					Signed: true,
					Size:   operandType.Size,
				}
			}
			return operandType
		}, zeus_value.IsNumberType)
	case InstrTypeCondJmp:
		p.tcCondJmp(tc, instr)
	case InstrTypeLoad:
		p.tcLoad(instr)
	case InstrTypeStore:
		p.tcStore(tc, instr)
	case InstrTypeReturn:
		p.tcReturn(tc, instr)
	case InstrTypeCallFunc:
		p.tcCallFunc(tc, instr)
	case InstrTypeIndirectFuncCall:
		p.tcIndirectFuncCall(tc, instr)
	case InstrTypeExport:
		p.tcExport(tc, instr)
	case InstrTypeImport:
		p.tcImport(tc, instr)
	case InstrTypeDeclClass:
		p.tcDeclClass(tc, instr)
	case InstrTypeNewObj:
		p.tcNewObj(tc, instr)
	case InstrTypeObjectPropertyAccess:
		p.tcObjectPropertyAccess(tc, instr)
	case InstrTypeDeclClassMethod:
		p.tcDeclClassMethod(tc, instr)
	case InstrTypeCast:
		// TODO: add type checking for cast
	default:
		panic(fmt.Sprintf("type checking not handled for instruction: %s", instr.Type))
	}
}

// validateFunctionReturns checks if a function returns in all code paths
func (p *TypeCheckingPass) validateFunctionReturns(tc *TypeChecker, function *zeus_value.Function, functionBody *BasicBlock) {
	returnsInAllBlocks := true
	worklist := []*BasicBlock{functionBody}

	for len(worklist) > 0 {
		block := worklist[0]

		if len(block.Instrs) == 0 || !IsControlFlowInstr(block.Instrs[len(block.Instrs)-1].Type) {
			// add implicit return
			if zeus_value.IsVoidType(function.ReturnType) {
				tc.builder.SetInsertionBlock(block)
				tc.builder.BuildReturn(nil, nil)
			} else {
				returnsInAllBlocks = false
			}
		}

		worklist = worklist[1:]
		worklist = append(worklist, block.Successors...)
	}

	if !returnsInAllBlocks {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("function '%s' does not return value of type '%s' in all paths", function.Name, function.ReturnType),
			Span:    function.Span,
		})
	}
}

func (p *TypeCheckingPass) tcFuncDecl(tc *TypeChecker, instr *Instr) {
	input := AsDeclFuncInstrInput(instr.Input)

	if input.Function.Name == token.MAIN_FUNCTION_NAME && tc.IsEntryPoint {
		p.hasMainFunction = true
	}

	p.validateFunctionReturns(tc, input.Function, input.Body)
}

func (p *TypeCheckingPass) tcDeclVar(tc *TypeChecker, instr *Instr) {
	decl_var := AsDeclVarInstrInput(instr.Input)

	if zeus_value.IsVoidType(decl_var.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot declare variable of type '%s'", decl_var.Variable.ValueType),
			Span:    decl_var.Variable.Span,
		})
	} else if decl_var.Initializer != nil {
		initializer := p.cmpValueWithImplicitCast(tc, instr, decl_var.Variable.ValueType, decl_var.Initializer)
		decl_var.Initializer = initializer
	}
}

func (p *TypeCheckingPass) cmpValueWithImplicitCast(tc *TypeChecker, instr *Instr, targetType zeus_value.ValueType, b zeus_value.Value) zeus_value.Value {
	bType := tc.getValueType(b)

	if !p.cmpValueType(tc, targetType, bType) {
		castedB, ok := p.tryImplicitCast(tc, instr, b, targetType)

		if ok {
			return castedB
		} else {
			if bType == nil {
				bType = zeus_value.UndefinedType{}
			}
			if targetType == nil {
				targetType = zeus_value.UndefinedType{}
			}
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", bType.String(), targetType.String()),
				Span:    instr.Span,
			})
		}
	}

	return b
}

func (p *TypeCheckingPass) cmpValueType(tc *TypeChecker, a, b zeus_value.ValueType) bool {
	switch a := a.(type) {
	case zeus_value.IntType:
		b, ok := b.(zeus_value.IntType)

		return ok && a.Signed == b.Signed && a.Size == b.Size
	case zeus_value.FloatType:
		bFloat, okFloat := b.(zeus_value.FloatType)
		return okFloat && a.Size == bFloat.Size
	case zeus_value.BoolType:
		_, ok := b.(zeus_value.BoolType)
		if !ok {
			return false
		}
		return true
	case zeus_value.FunctionType:
		b, ok := b.(zeus_value.FunctionType)
		if !ok || len(a.ParamTypes) != len(b.ParamTypes) {
			return false
		}

		isReturnTypeEqual := p.cmpValueType(tc, a.ReturnType, b.ReturnType)

		if !isReturnTypeEqual {
			return false
		}

		for i := range a.ParamTypes {
			if !p.cmpValueType(tc, a.ParamTypes[i], b.ParamTypes[i]) {
				return false
			}
		}

		return true
	
	case zeus_value.ObjectType:
	  bClassType, ok := b.(zeus_value.ObjectType)
		return ok && a.Class.Name == bClassType.Class.Name
	}

	return false
}

// Performs the following implicit casts:
// - int to float
// - int to int of bigger size
// - float to float of bigger size
func (p *TypeCheckingPass) tryImplicitCast(tc *TypeChecker, instr *Instr, value zeus_value.Value, targetType zeus_value.ValueType) (zeus_value.Value, bool) {
	valueType := tc.getValueType(value)

	// if they both are same type, no need to cast
	if p.cmpValueType(tc, valueType, targetType) {
		return value, true
	}

	zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
	tc.builder.SetBlockInsertionBefore(tc.currentBlock, instr)

	castIntToFloat := func(intType zeus_value.IntType, value zeus_value.Value) zeus_value.Value {
		size := zeus_value.F32
		if intType.Size == zeus_value.I64 {
			size = zeus_value.F64
		}

		return tc.builder.BuildCast(value, zeus_value.FloatType{Size: size}, value.GetSpan())
	}

	switch valueType := valueType.(type) {
	case zeus_value.IntType:
		switch targetType := targetType.(type) {
		case zeus_value.IntType:
			// value and target are signed and the target is larger
			canFitValue := targetType.Size > valueType.Size && valueType.Signed == targetType.Signed
			// value is unsigned and target is signed and the target is larger
			canFitUnsigned := targetType.Signed && !valueType.Signed && targetType.Size > valueType.Size
			// value is a constant and the target is larger
			constant := zeus_value.AsConstant(value)
			canFitUnsignedConstant := constant != nil && targetType.Size >= zeus_value.GetSignedIntSize(constant.Value)

			if canFitValue || canFitUnsigned || canFitUnsignedConstant {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		case zeus_value.FloatType:
			return castIntToFloat(valueType, value), true
		}
	case zeus_value.FloatType:
		switch targetType := targetType.(type) {
		case zeus_value.FloatType:
			if targetType.Size > valueType.Size {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		}
	}

	return value, false
}

// converts left and right to the same type
func (p *TypeCheckingPass) doImplicitCastToSameType(tc *TypeChecker, instr *Instr, left, right zeus_value.Value) (zeus_value.Value, zeus_value.Value) {
	leftValueType := tc.getValueType(left)
	rightValueType := tc.getValueType(right)
	castErrMsg := fmt.Sprintf("cannot do implicit cast to same type: %s and %s", leftValueType, rightValueType)
	ok := false

	switch leftValueType := leftValueType.(type) {
	case zeus_value.IntType:
		switch rightValueType := rightValueType.(type) {
		case zeus_value.IntType:
			if leftValueType.Size > rightValueType.Size {
				right, ok = p.tryImplicitCast(tc, instr, right, leftValueType)
				if !ok {
					tc.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("cannot cast %s to %s without an explicit cast", rightValueType, leftValueType),
						Span:    instr.Span,
					})
				}
			} else if rightValueType.Size > leftValueType.Size {
				left, ok = p.tryImplicitCast(tc, instr, left, rightValueType)
				if !ok {
					tc.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("cannot cast %s to %s without an explicit cast", leftValueType, rightValueType),
						Span:    instr.Span,
					})
				}
			}
		case zeus_value.FloatType:
			left, ok = p.tryImplicitCast(tc, instr, left, rightValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			tc.pushError(&zeus_error.ZeusError{
				Message: castErrMsg,
				Span:    instr.Span,
			})
		}
	case zeus_value.FloatType:
		switch rightValueType := rightValueType.(type) {
		case zeus_value.FloatType:
			if leftValueType.Size > rightValueType.Size {
				right, ok = p.tryImplicitCast(tc, instr, right, leftValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			} else if rightValueType.Size > leftValueType.Size {
				left, ok = p.tryImplicitCast(tc, instr, left, rightValueType)
				zeus_error.Assert(ok, "failed to cast smaller float to larger float")
			}
		case zeus_value.IntType:
			right, ok = p.tryImplicitCast(tc, instr, right, leftValueType)
			zeus_error.Assert(ok, "failed to cast int to float")
		default:
			panic(castErrMsg)
		}
	}

	return left, right
}

func (p *TypeCheckingPass) tcBinaryOp(tc *TypeChecker, instr *Instr, resultTypeFn func(a, b zeus_value.ValueType) zeus_value.ValueType, cmpTypeFn func(a, b zeus_value.ValueType) bool) {
	input := AsBinaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operands of type '%s' and '%s' for binary operation", tc.getValueType(input.Left), tc.getValueType(input.Right)),
			Span:    instr.Span,
		})
	} else {
		left, right := p.doImplicitCastToSameType(tc, instr, input.Left, input.Right)
		input.Left = left
		input.Right = right
	}

	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Left), tc.getValueType(input.Right))
}

func (p *TypeCheckingPass) tcUnaryOp(tc *TypeChecker, instr *Instr, resultTypeFn func(a zeus_value.ValueType) zeus_value.ValueType, cmpTypeFn func(a zeus_value.ValueType) bool) {
	input := AsUnaryOpInstrInput(instr.Input)

	if !cmpTypeFn(tc.getValueType(input.Value)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("invalid operand of type '%s' for unary operation", tc.getValueType(input.Value)),
			Span:    instr.Span,
		})
	}

	instr.Output.ValueType = resultTypeFn(tc.getValueType(input.Value))
}

func (p *TypeCheckingPass) tcCondJmp(tc *TypeChecker, instr *Instr) {
	input := AsCondJmpInstrInput(instr.Input)

	if !zeus_value.IsBoolType(tc.getValueType(input.Condition)) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("condition must be of type bool, but found %s", tc.getValueType(input.Condition)),
			Span:    instr.Span,
		})
	}
}

func (p *TypeCheckingPass) tcLoad(instr *Instr) {
	input := AsLoadInstrInput(instr.Input)
	instr.Output.ValueType = input.Addr.ValueType
}

func (p *TypeCheckingPass) tcStore(tc *TypeChecker, instr *Instr) {
	input := AsStoreInstrInput(instr.Input)

	if !input.Addr.IsPtr {
		tc.pushError(&zeus_error.ZeusError{
			Message: "invalid lvalue in assignment",
			Span:    instr.Span,
		})
	}

	input.Value = p.cmpValueWithImplicitCast(tc, instr, input.Addr.ValueType, input.Value)
}

func (p *TypeCheckingPass) tcReturn(tc *TypeChecker, instr *Instr) {
	input := AsReturnInstrInput(instr.Input)

	if tc.currentFunction == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "return statement outside of function",
			Span:    instr.Span,
		})

		return
	}

	returnType := tc.currentFunction.ReturnType

	if zeus_value.IsVoidType(returnType) && input.Value != nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot return a value from void function",
			Span:    instr.Span,
		})
	} else if !zeus_value.IsVoidType(returnType) && input.Value == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("return value of type '%s' is expected", tc.getValueType(input.Value)),
			Span:    instr.Span,
		})
	} else if zeus_value.IsVoidType(returnType) && input.Value == nil {
		return
	} else {
		input.Value = p.cmpValueWithImplicitCast(tc, instr, returnType, input.Value)
	}
}

func (p *TypeCheckingPass) tcExport(tc *TypeChecker, instr *Instr) {
	input := AsExportInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if !zeus_value.IsFunctionType(valueType) && !zeus_value.IsClass(input.Value) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot export value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
}

func (p *TypeCheckingPass) tcImport(tc *TypeChecker, instr *Instr) {
	input := AsImportInstrInput(instr.Input)
	valueType := tc.getValueType(input.Value)

	if !zeus_value.IsFunctionType(valueType) && !zeus_value.IsClass(input.Value) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot import value of type '%s'", valueType),
			Span:    instr.Span,
		})
	}
}

// tcFunctionCall performs common type checking logic for function calls
// It validates arguments and performs implicit casting based on function signature
func (p *TypeCheckingPass) tcFunctionCall(tc *TypeChecker, instr *Instr, functionType zeus_value.FunctionType, args []zeus_value.Value, calleeSpan *token.Span) []zeus_value.Value {
	if len(args) != len(functionType.ParamTypes) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected %d arguments for function, but found %d", len(functionType.ParamTypes), len(args)),
			Span:    calleeSpan,
		})
		return args
	}

	// Perform implicit casting on arguments
	for i := range args {
		castedArg, ok := p.tryImplicitCast(tc, instr, args[i], functionType.ParamTypes[i])
		args[i] = castedArg
		if !ok {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("argument %d of type '%s' does not match expected type '%s'", i+1, tc.getValueType(args[i]), functionType.ParamTypes[i]),
				Span:    args[i].GetSpan(),
			})
		}
	}

	return args
}

func (p *TypeCheckingPass) tcCallFunc(tc *TypeChecker, instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	function := zeus_value.AsFunction(input.Callee)

	if function == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    input.Callee.GetSpan(),
		})

		return
	}

	functionType := zeus_value.ToFunctionType(*function)

	// Use the abstracted function call type checking
	input.Args = p.tcFunctionCall(tc, instr, functionType, input.Args, input.Callee.GetSpan())
	instr.Input = NewCallFuncInstrInput(input.Callee, input.Args)

	instr.Output.ValueType = functionType.ReturnType
}

func (p *TypeCheckingPass) tcDeclClass(tc *TypeChecker, instr *Instr) {}

func (p *TypeCheckingPass) tcNewObj(tc *TypeChecker, instr *Instr) {
	input := AsNewObjInstrInput(instr.Input)

	if !zeus_value.IsClass(input.Callee) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot create object of type '%s'", tc.getValueType(input.Callee)),
			Span:    input.Callee.GetSpan(),
		})
	}

	class := zeus_value.AsClass(input.Callee)

	var constructorMethod *zeus_value.Function = nil
	var constructorAccessModifier token.TokenType = token.TokenTypePublic

	for _, method := range class.Methods {
		if method.Method.Name == token.CONSTRUCTOR_METHOD_NAME {
			constructorMethod = method.Method
			if method.AccessModifier != nil {
				constructorAccessModifier = method.AccessModifier.Type
			}
			break
		}
	}
	

	if constructorMethod == nil && len(input.Args) > 0 {
		tc.pushError(&zeus_error.ZeusError{
			Message: "no constructor method found in class",
			Span:    instr.Output.Span,
		})
	}

	if constructorMethod != nil {
		if constructorAccessModifier == token.TokenTypePrivate {
			tc.pushError(&zeus_error.ZeusError{
				Message: "cannot create object of class with private constructor",
				Span:    instr.Output.Span,
			})
		} else if len(input.Args) != len(constructorMethod.Params) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("expected %d arguments for constructor, but found %d", len(constructorMethod.Params), len(input.Args)),
				Span:    instr.Output.Span,
			})
		} else {
			for i := range input.Args {
				input.Args[i] = p.cmpValueWithImplicitCast(tc, instr, constructorMethod.Params[i].ValueType, input.Args[i])
			}
		}
	}

	instr.Output.ValueType = zeus_value.NewObjectType(*class)
}

func (p *TypeCheckingPass) tcObjectPropertyAccess(tc *TypeChecker, instr *Instr) {
	input := AsObjectPropertyAccessInstrInput(instr.Input)
	output := instr.Output
	objectType := tc.getValueType(input.Object)

	if !zeus_value.IsObjectType(objectType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot access property %s of type '%s'", input.Property, objectType),
			Span:   output.Span,
		})
	} else {
		class := zeus_value.AsObjectType(objectType).Class
		properties := class.Properties
		methods := class.Methods
		isFound := false
		isAccessible := false

		for _, property := range properties {
			if property.Property.Name == input.Property {
				isFound = true
				if property.AccessModifier != nil {
					isAccessible = property.AccessModifier.Type == token.TokenTypePublic
				}
				instr.Output.ValueType = property.Property.ValueType
			}
		}

		for _, method := range methods {
			if method.Method.Name == input.Property {
				isFound = true
				if method.AccessModifier != nil {
					isAccessible = method.AccessModifier.Type == token.TokenTypePublic
				}
				instr.Output.ValueType = zeus_value.ToFunctionType(*method.Method)
			}
		}

		if !isFound {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("property %s not found in class %s", input.Property, class.Name),
				Span:    output.Span,
			})
		}

		if input.Property == token.CONSTRUCTOR_METHOD_NAME {
			tc.pushError(&zeus_error.ZeusError{
				Message: "cannot access constructor method of a class",
				Span:    output.Span,
			})
		} else {
			propertyOfSameClass := tc.currentClass != nil && tc.currentClass.Name == class.Name
			if !isAccessible && !propertyOfSameClass {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("property %s is not accessible in class %s", input.Property, class.Name),
					Span:    output.Span,
				})
			}
		}
	}
}

func (p *TypeCheckingPass) tcIndirectFuncCall(tc *TypeChecker, instr *Instr) {
	input := AsIndirectFuncCallInstrInput(instr.Input)

	methodType := tc.getValueType(input.Function)
	functionType := zeus_value.AsFunctionType(methodType)

	if functionType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    instr.Output.Span,
		})
		return
	}

	input.Args = p.tcFunctionCall(tc, instr, *functionType, input.Args, input.Function.GetSpan())
	instr.Input = NewIndirectFuncCallInstrInput(input.Function, input.Args)

	instr.Output.ValueType = functionType.ReturnType
}
 
func (p *TypeCheckingPass) tcDeclClassMethod(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassMethodInstrInput(instr.Input)
	p.validateFunctionReturns(tc, input.Method, input.Body)
}


// UnusedWarningPass generates warnings for unused identifiers
// This pass should run last after all usage tracking is complete
type UnusedWarningPass struct{}

func NewUnusedWarningPass() *UnusedWarningPass {
	return &UnusedWarningPass{}
}

func (p *UnusedWarningPass) GetName() string {
	return "UnusedWarningPass"
}

func (p *UnusedWarningPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	switch instr.Type {
	case InstrTypeDeclVar:
		p.handleVarDecl(instr)
	case InstrTypeStore:
		p.handleStore(instr)
	case InstrTypeLoad:
		p.handleLoad(instr)
	case InstrTypeAdd, InstrTypeSub, InstrTypeMul, InstrTypeDiv,
		 InstrTypeEqEq, InstrTypeNotEq, InstrTypeLessThan, InstrTypeGreaterThan,
		 InstrTypeLessThanEq, InstrTypeGreaterThanEq:
		p.handleBinaryOp(instr)
	case InstrTypeNot, InstrTypeNeg:
		p.handleUnaryOp(instr)
	case InstrTypeCallFunc:
		p.handleCallFunc(instr)
	case InstrTypeIndirectFuncCall:
		p.handleIndirectFuncCall(instr)
	case InstrTypeReturn:
		p.handleReturn(instr)
	case InstrTypeCondJmp:
		p.handleCondJmp(instr)
	case InstrTypeObjectPropertyAccess:
		p.handleObjectPropertyAccess(tc, instr)
	case InstrTypeExport:
		p.handleExport(instr)
	}
}

// handleVarDecl processes variable declarations and marks initializers as used
func (p *UnusedWarningPass) handleVarDecl(instr *Instr) {
	input := AsDeclVarInstrInput(instr.Input)
	
	// If the variable has an initializer, mark it as used
	if input.Initializer != nil {
		// If the initializer is a variable or function, mark it as used
		if initVar := zeus_value.AsVar(input.Initializer); initVar != nil {
			initVar.IsUsed = true
		} else if initFunc := zeus_value.AsFunction(input.Initializer); initFunc != nil {
			initFunc.IsUsed = true
		}
	}
}

// handleStore processes variable assignments and marks values as used
func (p *UnusedWarningPass) handleStore(instr *Instr) {
	input := AsStoreInstrInput(instr.Input)
	
	// If the value being stored is a variable or function, mark it as used
	if valueVar := zeus_value.AsVar(input.Value); valueVar != nil {
		valueVar.IsUsed = true
	} else if valueFunc := zeus_value.AsFunction(input.Value); valueFunc != nil {
		valueFunc.IsUsed = true
	}
}

// handleLoad processes variable usage and marks variables as used
func (p *UnusedWarningPass) handleLoad(instr *Instr) {
	input := AsLoadInstrInput(instr.Input)
	
	// Mark the variable as used
	input.Addr.IsUsed = true
}

// handleBinaryOp processes binary operations and marks operands as used
func (p *UnusedWarningPass) handleBinaryOp(instr *Instr) {
	input := AsBinaryOpInstrInput(instr.Input)
	
	// Mark the left operand as used
	if leftVar := zeus_value.AsVar(input.Left); leftVar != nil {
		leftVar.IsUsed = true
	} else if leftFunc := zeus_value.AsFunction(input.Left); leftFunc != nil {
		leftFunc.IsUsed = true
	}
	
	// Mark the right operand as used
	if rightVar := zeus_value.AsVar(input.Right); rightVar != nil {
		rightVar.IsUsed = true
	} else if rightFunc := zeus_value.AsFunction(input.Right); rightFunc != nil {
		rightFunc.IsUsed = true
	}
}

// handleUnaryOp processes unary operations and marks the operand as used
func (p *UnusedWarningPass) handleUnaryOp(instr *Instr) {
	input := AsUnaryOpInstrInput(instr.Input)
	
	// Mark the operand as used
	if operandVar := zeus_value.AsVar(input.Value); operandVar != nil {
		operandVar.IsUsed = true
	} else if operandFunc := zeus_value.AsFunction(input.Value); operandFunc != nil {
		operandFunc.IsUsed = true
	}
}

// handleCallFunc processes function calls and marks arguments as used
func (p *UnusedWarningPass) handleCallFunc(instr *Instr) {
	input := AsCallFuncInstrInput(instr.Input)
	
	// Mark the callee as used (variable or function)
	if calleeVar := zeus_value.AsVar(input.Callee); calleeVar != nil {
		calleeVar.IsUsed = true
	} else if calleeFunc := zeus_value.AsFunction(input.Callee); calleeFunc != nil {
		calleeFunc.IsUsed = true
	}
	
	// Mark arguments as used
	for i := range input.Args {
		if argVar := zeus_value.AsVar(input.Args[i]); argVar != nil {
			argVar.IsUsed = true
		} else if argFunc := zeus_value.AsFunction(input.Args[i]); argFunc != nil {
			argFunc.IsUsed = true
		}
	}
}

// handleIndirectFuncCall processes indirect function calls and marks the function and arguments as used
func (p *UnusedWarningPass) handleIndirectFuncCall(instr *Instr) {
	input := AsIndirectFuncCallInstrInput(instr.Input)
	
	// Mark the function as used (variable or function)
	if funcVar := zeus_value.AsVar(input.Function); funcVar != nil {
		funcVar.IsUsed = true
	} else if function := zeus_value.AsFunction(input.Function); function != nil {
		function.IsUsed = true
	}
	
	// Mark arguments as used
	for i := range input.Args {
		if argVar := zeus_value.AsVar(input.Args[i]); argVar != nil {
			argVar.IsUsed = true
		} else if argFunc := zeus_value.AsFunction(input.Args[i]); argFunc != nil {
			argFunc.IsUsed = true
		}
	}
}

// handleReturn processes return statements and marks the return value as used
func (p *UnusedWarningPass) handleReturn(instr *Instr) {
	input := AsReturnInstrInput(instr.Input)
	
	// Mark the return value as used if it's not nil
	if returnValueVar := zeus_value.AsVar(input.Value); returnValueVar != nil {
		returnValueVar.IsUsed = true
	} else if returnValueFunc := zeus_value.AsFunction(input.Value); returnValueFunc != nil {
		returnValueFunc.IsUsed = true
	}
}

// handleCondJmp processes conditional jumps and marks the condition as used
func (p *UnusedWarningPass) handleCondJmp(instr *Instr) {
	input := AsCondJmpInstrInput(instr.Input)
	
	// Mark the condition as used
	if condVar := zeus_value.AsVar(input.Condition); condVar != nil {
		condVar.IsUsed = true
	}
}

// handleObjectPropertyAccess processes object property accesses and marks the object as used
// Also marks class methods and properties as used when they are accessed
func (p *UnusedWarningPass) handleObjectPropertyAccess(tc *TypeChecker, instr *Instr) {
	input := AsObjectPropertyAccessInstrInput(instr.Input)
	
	// Mark the object as used
	if objectVar := zeus_value.AsVar(input.Object); objectVar != nil {
		objectVar.IsUsed = true
	}
	
	// Mark class methods and properties as used when accessed
	objectType := tc.getValueType(input.Object)
	
	// Check if it's an object type (class instance)
	if zeus_value.IsObjectType(objectType) {
		objectTypeStruct := zeus_value.AsObjectType(objectType)
		class := objectTypeStruct.Class
		
		// Look for a method with the matching name
		for _, classMethod := range class.Methods {
			if classMethod.Method.Name == input.Property {
				// Mark the method as used
				classMethod.Method.IsUsed = true
				return
			}
		}
		
		// Look for a property with the matching name
		for _, classProperty := range class.Properties {
			if classProperty.Property.Name == input.Property {
				// Mark the property as used
				classProperty.Property.IsUsed = true
				return
			}
		}
	}
}

// handleExport processes export statements and marks exported functions as used
func (p *UnusedWarningPass) handleExport(instr *Instr) {
	input := AsExportInstrInput(instr.Input)
	
	// Mark exported functions as used
	if function := zeus_value.AsFunction(input.Value); function != nil {
		function.IsUsed = true
	}
}

func (p *UnusedWarningPass) Finalize(tc *TypeChecker) {
	// Check for unused variables, functions, and class methods - push warnings
	tc.builder.symbolTable.Walk(func(name string, value zeus_value.Value) {
		if variable := zeus_value.AsVar(value); variable != nil {
			// Skip temporary variables as they shouldn't generate warnings
			if !variable.IsTempVariable() && !variable.IsUsed {
				tc.pushError(&zeus_error.ZeusError{
					Severity: zeus_error.ErrorSeverityWarning,
					Message: fmt.Sprintf("identifier '%s' is declared but not used", variable.Name),
					Span:    variable.Span,
				})
			}
		} else if function := zeus_value.AsFunction(value); function != nil {
			isMainFunction := function.Name == token.MAIN_FUNCTION_NAME && tc.IsEntryPoint
			// Check for unused functions, ignore class methods
			if !function.IsUsed && !strings.Contains(function.Name, ".") && !isMainFunction {
				tc.pushError(&zeus_error.ZeusError{
					Severity: zeus_error.ErrorSeverityWarning,
					Message: fmt.Sprintf("function '%s' is declared but not used", function.Name),
					Span:    function.Span,
				})
			}
		} else if class := zeus_value.AsClass(value); class != nil {
			// Check all methods in this class
			for _, classMethod := range class.Methods {
				method := classMethod.Method
				
				// Skip constructor methods as they're implicitly used
				if method.Name == token.CONSTRUCTOR_METHOD_NAME {
					continue
				}
				
				// Check if the method is unused
				if !method.IsUsed {
					tc.pushError(&zeus_error.ZeusError{
						Severity: zeus_error.ErrorSeverityWarning,
						Message: fmt.Sprintf("method '%s' in class '%s' is declared but not used", method.Name, class.Name),
						Span:    method.Span,
					})
				}
			}
			
			// Check all properties in this class
			for _, classProperty := range class.Properties {
				property := classProperty.Property
				
				// Check if the property is unused
				if !property.IsUsed {
					tc.pushError(&zeus_error.ZeusError{
						Severity: zeus_error.ErrorSeverityWarning,
						Message: fmt.Sprintf("property '%s' in class '%s' is declared but not used", property.Name, class.Name),
						Span:    property.Span,
					})
				}
			}
		}
	})
}
