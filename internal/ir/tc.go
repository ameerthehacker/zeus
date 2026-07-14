package ir

import (
	"fmt"

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
	currentClass    *zeus_value.Class
	builder         *IRBuilder
	currentBlock    *BasicBlock
	passes          []TCPass
	IsEntryPoint    bool
}

func NewTypeChecker(builder *IRBuilder, isEntryPoint bool) *TypeChecker {
	tc := &TypeChecker{
		builder:      builder,
		IsEntryPoint: isEntryPoint,
	}

	// Initialize required passes
	tc.passes = []TCPass{
		NewTypeCheckingPass(),
		NewUnusedWarningPass(),
		NewUndefinedTypeCheckPass(),
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
		// Static methods emit InstrTypeDeclFunc (not InstrTypeDeclClassMethod).
		// Function.Class is set by buildStaticMethods/buildAccessors for statics; nil for regular fns.
		tc.currentClass = input.Function.Class
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
	// DECLARE_GLOBAL_VAR is a top-level declaration: ambient-global extern references are emitted at
	// module scope (outside any function), and it is otherwise only produced for module-scope vars.
	isTopLevelInstr := IsFunctionDeclInstr(instr.Type) || IsClassDeclInstr(instr.Type) || IsClassMethodDeclInstr(instr.Type) || IsExportInstr(instr.Type) || IsImportInstr(instr.Type) || IsPrimordialFunctionDeclInstr(instr.Type) || instr.Type == InstrTypeDeclGlobalVar
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
		return zeus_value.NewObjectType(value)
	default:
		if value == nil {
			return zeus_value.UndefinedType{}
		}
		return zeus_value.UndefinedType{Span: value.GetSpan()}
	}
}

// getValueType returns the value type of a value
func (tc *TypeChecker) getValueType(value zeus_value.Value) zeus_value.ValueType {
	if value == nil {
		return zeus_value.UndefinedType{}
	}

	valueType := tc.getBuiltInValueType(value)

	if valueType == nil {
		return zeus_value.UndefinedType{Span: value.GetSpan()}
	}

	return valueType
}

func (tc *TypeChecker) pushError(err *zeus_error.ZeusError) {
	tc.errors = append(tc.errors, err)
}

// protectedAccessAllowed reports the extra reach `protected` grants over `private`: a protected
// member reachable on an object of static type `objectClass` is accessible when the current class
// is that class or a subclass of it. Non-protected modifiers return false — callers handle
// public / private / same-class separately.
func (tc *TypeChecker) protectedAccessAllowed(mod *token.Token, objectClass *zeus_value.Class) bool {
	return mod != nil && mod.Type == token.TokenTypeProtected &&
		tc.currentClass != nil && objectClass != nil &&
		zeus_value.IsSubclassOf(tc.currentClass, objectClass)
}

// variadicElementType returns the value type each trailing argument of a variadic call
// must match, given the rest parameter's (array) type. A nested array element type is
// normalised to its object form so it compares against array-valued arguments.
func (tc *TypeChecker) variadicElementType(restParamType zeus_value.ValueType) zeus_value.ValueType {
	objType := zeus_value.AsObjectType(restParamType)
	if objType == nil {
		return restParamType
	}
	elementType := objType.Class.ArrayElementType
	if arrayType, ok := elementType.(zeus_value.ArrayType); ok {
		return zeus_value.NewObjectType(tc.getClassFromArrayType(arrayType))
	}
	return elementType
}

// getClassFromArrayType looks up (or creates on demand) the array primordial class.
func (tc *TypeChecker) getClassFromArrayType(arrayType zeus_value.ArrayType) *zeus_value.Class {
	arrayTypeClassName := arrayType.String()

	if class, ok := tc.builder.symbolTable.GetSymbol(arrayTypeClassName); ok {
		classValue, ok := class.(*zeus_value.Class)
		zeus_error.Assert(ok, fmt.Sprintf("array element type %s is not a class", arrayTypeClassName))
		return classValue
	}

	// Create on demand from registry (handles the case where emitFunction stored a raw
	// ArrayType annotation without calling resolveTypeForIRGen).
	arrayClass := zeus_value.Registry.GetOrCreateArrayClass(arrayType)
	tc.builder.symbolTable.DeclareGlobalSymbol(arrayTypeClassName, arrayClass)
	tc.builder.EmitClassDeclAtStart(arrayClass)
	return arrayClass
}
