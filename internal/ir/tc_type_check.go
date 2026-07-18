package ir

import (
	"fmt"

	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// This pass does the actual type checking
type TypeCheckingPass struct {
	hasMainFunction bool
	firstUserSpan   *token.Span
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
			Span:    p.firstUserSpan,
		})
	}
}

func (p *TypeCheckingPass) tcCast(tc *TypeChecker, instr *Instr) {
	input := AsCastInstrInput(instr.Input)
	sourceType := tc.getValueType(input.Value)

	// *Function → FunctionType: CastLoweringPass will wrap it in a functor class
	if zeus_value.AsFunction(input.Value) != nil {
		if _, ok := input.CastType.(zeus_value.FunctionType); ok {
			instr.Output.ValueType = input.CastType
			return
		}
	}

	if zeus_value.IsObjectType(sourceType) {
		if targetFnType, ok := input.CastType.(zeus_value.FunctionType); ok {
			p.tcFunctorCast(tc, instr, *zeus_value.AsObjectType(sourceType), targetFnType)
			return
		}
	}

	target := input.CastType

	// Explicit `as` (and implicit widening) numeric/bool casts: int/float/bool in any
	// direction, plus the C-numeric bridge (cint/clong/csize/cdouble ↔ Zeus scalars). Unchecked —
	// codegen's genCast truncates/wraps/reinterprets. cptr/cstr are deliberately excluded: they are
	// strict and only cross the FFI boundary or convert via the generic runtime helpers.
	if isCastableNumeric(sourceType) && isCastableNumeric(target) {
		instr.Output.ValueType = target
		return
	}

	// cptr <-> cstr: both are raw addrspace(0) pointers (void* / char*), freely interchangeable at
	// the FFI boundary. The cast is an identity in codegen. (Numeric<->pointer punning stays illegal.)
	if isCPointerType(sourceType) && isCPointerType(target) {
		instr.Output.ValueType = target
		return
	}

	// Existing implicit string ↔ u8[] casts are emitted as CAST and lowered in a later pass.
	if (isStringType(sourceType) || isU8ArrayType(sourceType)) &&
		(isStringType(target) || isU8ArrayType(target)) {
		return
	}

	// Object → object: legal only within one class hierarchy (up- or down-cast). Unrelated
	// classes are rejected at compile time. The runtime downcast check (INSTANCEOF + throw) is
	// emitted during IR gen (VisitCastExpr); here we only validate the static relationship.
	if srcObj := zeus_value.AsObjectType(sourceType); srcObj != nil {
		if dstObj, ok := target.(zeus_value.ObjectType); ok {
			if zeus_value.IsSubclassOf(srcObj.Class, dstObj.Class) || zeus_value.IsSubclassOf(dstObj.Class, srcObj.Class) {
				instr.Output.ValueType = target
				return
			}
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("cannot cast '%s' to '%s': unrelated class types", sourceType, target),
				Span:    instr.Span,
			})
			return
		}
	}

	// Anything else is an illegal cast, reported at compile time (zero runtime cost).
	tc.pushError(&zeus_error.ZeusError{
		Message: fmt.Sprintf("cannot cast '%s' to '%s'", sourceType, target),
		Span:    instr.Span,
	})
}

// tcInstanceOf type-checks a runtime type test (object `as` downcast guard). The output is a bool.
func (p *TypeCheckingPass) tcInstanceOf(tc *TypeChecker, instr *Instr) {
	input := AsInstanceOfInstrInput(instr.Input)
	_ = tc.getValueType(input.Value) // ensure the operand resolves
	instr.Output.ValueType = zeus_value.BoolType{Span: instr.Span}
}

// isNumericOrBoolType reports whether a type participates in unchecked numeric/bool `as` casts.
func isNumericOrBoolType(valueType zeus_value.ValueType) bool {
	switch valueType.(type) {
	case zeus_value.IntType, zeus_value.FloatType, zeus_value.BoolType:
		return true
	}
	return false
}

// isCastableNumeric extends isNumericOrBoolType with the C-numeric types (cint/clong/csize/cdouble),
// which bridge to/from Zeus scalars via `as`. Pointer C types (cptr/cstr) are excluded — strict.
func isCastableNumeric(valueType zeus_value.ValueType) bool {
	return isNumericOrBoolType(valueType) || zeus_value.IsCNumericType(valueType)
}

// isCPointerType reports whether a type is a raw C pointer (cptr or cstr).
func isCPointerType(valueType zeus_value.ValueType) bool {
	c, ok := valueType.(zeus_value.CType)
	return ok && (c.Kind == zeus_value.CPtr || c.Kind == zeus_value.CStr)
}

func (p *TypeCheckingPass) tcCoerce(tc *TypeChecker, instr *Instr) {
	input := AsCoerceInstrInput(instr.Input)
	sourceType := tc.getValueType(input.Value)
	objType := zeus_value.AsObjectType(sourceType)
	if objType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("coerce source must be an object type, got '%s'", sourceType),
			Span:    instr.Span,
		})
		return
	}
	targetFnType, ok := input.TargetType.(zeus_value.FunctionType)
	if !ok {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("coerce target must be a function type, got '%s'", input.TargetType),
			Span:    instr.Span,
		})
		return
	}
	p.tcFunctorCast(tc, instr, *objType, targetFnType)
	instr.Output.ValueType = sourceType
}

func (p *TypeCheckingPass) tcFunctorCast(tc *TypeChecker, instr *Instr, sourceClass zeus_value.ObjectType, targetFnType zeus_value.FunctionType) {
	span := instr.Span

	var callMethod *zeus_value.Function
	for _, m := range sourceClass.Class.Methods {
		// A nil access modifier means the default (public).
		isPublic := m.AccessModifier == nil || m.AccessModifier.Type == token.TokenTypePublic
		if m.Method.SourceName() == token.FUNCTOR_CALL_METHOD_NAME && isPublic {
			callMethod = m.Method
			m.Method.IsUsed = true
			break
		}
	}

	if callMethod == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot cast '%s' to function type: no public '%s' method found",
				sourceClass.Class.SourceName(), token.FUNCTOR_CALL_METHOD_NAME),
			Span: span,
		})
		return
	}

	callFnType := zeus_value.AsFunctionType(zeus_value.GetValueType(callMethod))
	if callFnType == nil {
		return
	}

	if len(callFnType.ParamTypes) != len(targetFnType.ParamTypes) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot cast '%s' to '%s': '%s' has %d parameter(s) but function type expects %d",
				sourceClass.Class.SourceName(), targetFnType, token.FUNCTOR_CALL_METHOD_NAME,
				len(callFnType.ParamTypes), len(targetFnType.ParamTypes)),
			Span: span,
		})
		return
	}

	for i, callParamType := range callFnType.ParamTypes {
		if !zeus_value.CmpValueType(callParamType, targetFnType.ParamTypes[i]) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("cannot cast '%s' to '%s': parameter %d type mismatch ('%s' vs '%s')",
					sourceClass.Class.SourceName(), targetFnType, i+1, callParamType, targetFnType.ParamTypes[i]),
				Span: span,
			})
			return
		}
	}

	if !zeus_value.CmpValueType(callFnType.ReturnType, targetFnType.ReturnType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot cast '%s' to '%s': return type mismatch ('%s' vs '%s')",
				sourceClass.Class.SourceName(), targetFnType, callFnType.ReturnType, targetFnType.ReturnType),
			Span: span,
		})
	}
}

func (p *TypeCheckingPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	// Track the first span from user-written code (skip primordial declarations)
	if p.firstUserSpan == nil && instr.Span != nil && instr.Type != InstrTypeDeclPrimordialFunc {
		p.firstUserSpan = instr.Span
	}

	switch instr.Type {
	// jmp requires no type checking
	case InstrTypeJmp:
	case InstrTypeDeclFunc:
		p.tcFuncDecl(tc, instr)
	case InstrTypeDeclVar, InstrTypeDeclGlobalVar:
		p.tcDeclVar(tc, instr)
	case InstrTypeAdd:
		p.tcBinaryOp(tc, instr, func(a, b zeus_value.ValueType) zeus_value.ValueType {
			// String + string returns string
			if isStringType(a) && isStringType(b) {
				return a
			}
			return zeus_value.GetBiggerType(a, b)
		}, func(a, b zeus_value.ValueType) bool {
			// Allow number + number
			if zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b) {
				return true
			}
			// Allow string + string (concatenation)
			if isStringType(a) && isStringType(b) {
				return true
			}
			return false
		})
	case InstrTypeSub:
		fallthrough
	case InstrTypeMul:
		fallthrough
	case InstrTypeDiv:
		p.tcBinaryOp(tc, instr, zeus_value.GetBiggerType, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
		})
	case InstrTypeMod:
		// Modulo only works on integers
		p.tcBinaryOp(tc, instr, zeus_value.GetBiggerType, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsIntType(a) && zeus_value.IsIntType(b)
		})
	case InstrTypePower:
		// Power works on numeric types (result is float if either operand is float)
		p.tcBinaryOp(tc, instr, func(a, b zeus_value.ValueType) zeus_value.ValueType {
			// If either is a float, result is float
			if zeus_value.IsFloatType(a) || zeus_value.IsFloatType(b) {
				return zeus_value.GetBiggerType(a, b)
			}
			// Integer power: for now, return the type of the base
			// In practice, we might want to promote to f64 for safety
			return a
		}, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b)
		})
	case InstrTypeEqEq:
		fallthrough
	case InstrTypeNotEq:
		p.tcBinaryOp(tc, instr, func(_, _ zeus_value.ValueType) zeus_value.ValueType {
			return zeus_value.BoolType{}
		}, func(a, b zeus_value.ValueType) bool {
			// Allow number comparisons
			if zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b) {
				return true
			}
			// Allow object/interface values (which are object pointers at runtime) to be
			// compared with null and with each other.
			isRef := func(t zeus_value.ValueType) bool {
				return zeus_value.IsObjectType(t) || zeus_value.IsInterfaceType(t)
			}
			if isRef(a) && zeus_value.IsNullType(b) {
				return true
			}
			if zeus_value.IsNullType(a) && isRef(b) {
				return true
			}
			// Allow two object/interface types to be compared
			if isRef(a) && isRef(b) {
				return true
			}
			// Allow boolean comparisons
			if zeus_value.IsBoolType(a) && zeus_value.IsBoolType(b) {
				return true
			}
			// Allow string comparisons
			if isStringType(a) && isStringType(b) {
				return true
			}
			// Allow function pointer compared with null (used by null-check lowering pass)
			if zeus_value.IsFunctionType(a) && zeus_value.IsNullType(b) {
				return true
			}
			if zeus_value.IsNullType(a) && zeus_value.IsFunctionType(b) {
				return true
			}
			return false
		})
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
			// Allow number comparisons
			if zeus_value.IsNumberType(a) && zeus_value.IsNumberType(b) {
				return true
			}
			// Allow string comparisons
			if isStringType(a) && isStringType(b) {
				return true
			}
			return false
		})
	case InstrTypeNot:
		p.tcUnaryOp(tc, instr, func(_ zeus_value.ValueType) zeus_value.ValueType {
			return zeus_value.BoolType{}
		}, zeus_value.IsBoolType)
	case InstrTypeAnd:
		fallthrough
	case InstrTypeOr:
		// Logical AND/OR: both operands must be bool, result is bool
		p.tcBinaryOp(tc, instr, func(_, _ zeus_value.ValueType) zeus_value.ValueType {
			return zeus_value.BoolType{}
		}, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsBoolType(a) && zeus_value.IsBoolType(b)
		})
	case InstrTypeBitAnd:
		fallthrough
	case InstrTypeBitOr:
		fallthrough
	case InstrTypeBitXor:
		fallthrough
	case InstrTypeShl:
		fallthrough
	case InstrTypeShr:
		// Bitwise ops: both operands must be integers, result is the bigger int type
		p.tcBinaryOp(tc, instr, zeus_value.GetBiggerType, func(a, b zeus_value.ValueType) bool {
			return zeus_value.IsIntType(a) && zeus_value.IsIntType(b)
		})
	case InstrTypeBitNot:
		// Bitwise NOT: operand must be integer, result is same type
		p.tcUnaryOp(tc, instr, func(operandType zeus_value.ValueType) zeus_value.ValueType {
			return operandType
		}, zeus_value.IsIntType)
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
	case InstrTypeCallModuleInit:
		// Calls a module's init function by symbol name; no operands to type check.
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
	case InstrTypeBox:
		p.tcBox(instr)
	case InstrTypeUnbox:
		p.tcUnbox(instr)
	case InstrTypeObjectPropertyAccess:
		p.tcObjectPropertyAccess(tc, instr)
	case InstrTypeMethodCall:
		p.tcMethodCall(tc, instr)
	case InstrTypeSuperConstructorCall:
		p.tcSuperConstructorCall(tc, instr)
	case InstrTypeDeclClassMethod:
		p.tcDeclClassMethod(tc, instr)
	case InstrTypeCast:
		p.tcCast(tc, instr)
	case InstrTypeInstanceOf:
		p.tcInstanceOf(tc, instr)
	case InstrTypeCoerce:
		p.tcCoerce(tc, instr)
	case InstrTypeDeclPrimordialFunc:
		// no type checking for primordial functions
	case InstrTypeGetAccessor:
		p.tcGetAccessor(tc, instr)
	case InstrTypeSetAccessor:
		p.tcSetAccessor(tc, instr)
	case InstrTypeGetIndex:
		p.tcGetIndex(tc, instr)
	case InstrTypeSetIndex:
		p.tcSetIndex(tc, instr)
	// Exception handling instructions
	case InstrTypeThrow:
		p.tcThrow(tc, instr)
	case InstrTypePushHandler:
		// Push handler doesn't require type checking
	case InstrTypePopHandler:
		// Pop handler doesn't require type checking
	case InstrTypeCheckException:
		// Check exception doesn't require type checking
	case InstrTypeGetException:
		// Get exception sets output type to the exception pointer type
		// This will be resolved when binding to catch variable
	case InstrTypeClearException:
		// Clear exception doesn't require type checking
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

	if (input.Function.Name == token.MAIN_FUNCTION_NAME && tc.IsEntryPoint) || input.Function.IsOSEntry {
		p.hasMainFunction = true
	}

	p.validateFunctionReturns(tc, input.Function, input.Body)
}

func (p *TypeCheckingPass) tcDeclVar(tc *TypeChecker, instr *Instr) {
	varDecl := AsDeclVarInstrInput(instr.Input)

	if zeus_value.IsVoidType(varDecl.Variable.ValueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot declare variable of type '%s'", varDecl.Variable.ValueType),
			Span:    varDecl.Variable.Span,
		})
	} else if varDecl.Initializer != nil {
		// infer the type from initializer
		if zeus_value.IsUndefinedType(varDecl.Variable.ValueType) {
			varDecl.Variable.ValueType = tc.getValueType(varDecl.Initializer)
		}
		initializer := p.cmpValueWithImplicitCast(tc, instr, varDecl.Variable.ValueType, varDecl.Initializer)
		varDecl.Initializer = initializer
	}
}

func (p *TypeCheckingPass) cmpValueWithImplicitCast(tc *TypeChecker, instr *Instr, targetType zeus_value.ValueType, b zeus_value.Value) zeus_value.Value {
	bType := tc.getValueType(b)

	// A raw function assigned to a function-type slot must be wrapped in a functor object,
	// even though its type already matches (all FunctionType values are functor objects at
	// runtime). This must run before the CmpValueType short-circuit below — same ordering
	// as tryImplicitCast — so property stores and returns wrap the function, not just var
	// initializers (which are wrapped during IR generation).
	if zeus_value.AsFunction(b) != nil {
		if _, ok := targetType.(zeus_value.FunctionType); ok {
			if casted, ok := p.tryImplicitCast(tc, instr, b, targetType); ok {
				return casted
			}
		}
	}

	if !zeus_value.CmpValueType(targetType, bType) {
		castedB, ok := p.tryImplicitCast(tc, instr, b, targetType)

		if ok {
			return castedB
		} else {
			if bType == nil {
				bType = zeus_value.UndefinedType{Span: b.GetSpan()}
			}
			if targetType == nil {
				targetType = zeus_value.UndefinedType{Span: b.GetSpan()}
			}
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("type '%s' is not assignable to type '%s'", bType.String(), targetType.String()),
				Span:    instr.Span,
			})
		}
	}

	return b
}

// tcBox is a defensive no-op: BOX is only emitted by this pass (emitBox) with its output type
// already set, so the walk never reaches an unset one. The case exists so a re-walk of an inserted
// BOX doesn't fall through to HandleInstruction's default panic.
func (p *TypeCheckingPass) tcBox(instr *Instr) {
	if instr.Output != nil && instr.Output.ValueType == nil {
		instr.Output.ValueType = zeus_value.NewObjectType(AsBoxInstrInput(instr.Input).TargetClass)
	}
}

// tcUnbox is a defensive no-op, mirroring tcBox: UNBOX is only emitted by this pass with its output
// type set, so this only guards against a re-walk hitting the default panic.
func (p *TypeCheckingPass) tcUnbox(instr *Instr) {
	if instr.Output != nil && instr.Output.ValueType == nil {
		box := AsUnboxInstrInput(instr.Input).Value
		if objType := zeus_value.AsObjectType(zeus_value.GetValueType(box)); objType != nil {
			instr.Output.ValueType = boxFieldType(objType.Class)
		}
	}
}

// emitBox autoboxes value (a scalar) into boxClass (Number/Bool), inserting the scalar→field cast
// (int/float → f64 for Number; boolean matches Bool's field exactly) and the BOX before instr. The
// builder's insertion point must already be positioned before instr.
func (p *TypeCheckingPass) emitBox(tc *TypeChecker, value zeus_value.Value, valueType zeus_value.ValueType, boxClass *zeus_value.Class, span *token.Span) zeus_value.Value {
	scalar := value
	if fieldType := boxFieldType(boxClass); fieldType != nil && !zeus_value.CmpValueType(valueType, fieldType) {
		scalar = tc.builder.BuildCast(value, fieldType, span)
	}
	return tc.builder.BuildBox(scalar, boxClass, span)
}

// Performs the following implicit casts:
// - int to float
// - int to int of bigger size
// - float to float of bigger size
// - primitive → Number/Bool (autobox) and primitive → interface satisfied by its box
func (p *TypeCheckingPass) tryImplicitCast(tc *TypeChecker, instr *Instr, value zeus_value.Value, targetType zeus_value.ValueType) (zeus_value.Value, bool) {
	valueType := tc.getValueType(value)

	// Object/interface → interface: assignability is a *directional* structural check
	// (CmpValueType is otherwise symmetric), so it must be tested target-first here. An
	// interface value is represented identically to the object, so no runtime cast is emitted.
	if zeus_value.IsInterfaceType(targetType) {
		if zeus_value.CmpValueType(targetType, valueType) {
			return value, true
		}
		// A primitive can satisfy an interface by first autoboxing into Number/Bool, when that box
		// structurally conforms to the interface (e.g. `let p: Printable = 5` where Number has the
		// interface's methods).
		if boxClass := boxClassForPrimitive(valueType); boxClass != nil &&
			zeus_value.CmpValueType(targetType, zeus_value.NewObjectType(boxClass)) {
			tc.builder.SetInsertionBeforeInstr(tc.currentBlock, instr)
			return p.emitBox(tc, value, valueType, boxClass, value.GetSpan()), true
		}
		return value, false
	}

	// *Function → FunctionType: wrap in a functor class (all FunctionType values are objects at runtime)
	// Must come before CmpValueType check since *Function.GetValueType() returns a matching FunctionType.
	if zeus_value.AsFunction(value) != nil {
		if _, ok := targetType.(zeus_value.FunctionType); ok {
			zeus_error.Assert(tc.currentBlock != nil, "current block is nil")
			tc.builder.SetBlockInsertionBefore(tc.currentBlock, instr)
			return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
		}
	}

	// if they both are same type, no need to cast
	if zeus_value.CmpValueType(valueType, targetType) {
		return value, true
	}

	tc.builder.SetInsertionBeforeInstr(tc.currentBlock, instr)

	// Any value whose type implements Stringify converts to `string` by calling toString()
	// (autoboxing a primitive first): powers console.log(5), `let s: string = x`, string params and
	// returns. The source is never itself a string here (that returns early above).
	if isStringType(targetType) {
		if converted, ok := p.emitToString(tc, value, valueType, value.GetSpan()); ok {
			return converted, true
		}
	}

	// Autobox a scalar into its box class when that exact primordial box is the target
	// (`let n: I32 = 5`, passing a bool where a Bool is expected). Compared by identity, not name, so
	// a user class that shadows a box name is not an autobox target (it fails assignability instead).
	if objTarget := zeus_value.AsObjectType(targetType); objTarget != nil {
		if boxClass := boxClassForPrimitive(valueType); boxClass != nil && objTarget.Class == boxClass {
			return p.emitBox(tc, value, valueType, boxClass, value.GetSpan()), true
		}
	}

	// Auto-unbox a boxed value flowing into a primitive slot. A `Number` (umbrella) unboxes to f64
	// via valueOf() — assigning to a narrower numeric type (`let x: i32 = n`) still needs an explicit
	// cast, the same no-silent-lossy-narrowing rule the language applies to any f64 -> i32. A concrete
	// box / Bool reads its exact field (`let flag: boolean = b`).
	if isNumberInterface(valueType) {
		if ft, ok := targetType.(zeus_value.FloatType); ok && ft.Size == zeus_value.F64 {
			return p.emitValueOf(tc, value, value.GetSpan()), true
		}
	}
	if srcObj := zeus_value.AsObjectType(valueType); srcObj != nil && isBoxedPrimordial(srcObj.Class) {
		fieldType := boxFieldType(srcObj.Class)
		if zeus_value.CmpValueType(fieldType, targetType) {
			return tc.builder.BuildUnbox(value, fieldType, value.GetSpan()), true
		}
	}

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
			// A literal constant adopts any int type whose range holds it — a compile-time retype
			// (no runtime cast), which also covers narrowing-that-fits like `let x: u8 = 200`.
			if c := zeus_value.AsConstant(value); c != nil && zeus_value.ConstantFitsInIntType(c.Value, targetType) {
				return zeus_value.NewConstant(c.Value, targetType, value.GetSpan()), true
			}
			// Non-constant widening: same signedness & larger, or unsigned→signed & larger.
			canFitValue := targetType.Size > valueType.Size && valueType.Signed == targetType.Signed
			canFitUnsigned := targetType.Signed && !valueType.Signed && targetType.Size > valueType.Size
			if canFitValue || canFitUnsigned {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		case zeus_value.FloatType:
			return castIntToFloat(valueType, value), true
		}
	case zeus_value.FloatType:
		switch targetType := targetType.(type) {
		case zeus_value.FloatType:
			// A float literal adopts the target float size (e.g. `let x: f32 = 2.0`).
			if c := zeus_value.AsConstant(value); c != nil {
				return zeus_value.NewConstant(c.Value, targetType, value.GetSpan()), true
			}
			if targetType.Size > valueType.Size {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		}
	case zeus_value.ArrayType:
		switch targetType := targetType.(type) {
		case zeus_value.ObjectType:
			// if the class name is stringifies version of value type, then it is a valid cast
			// This handles u8[] -> u8[] (as ObjectType) which is a no-op cast
			if targetType.Class.Name == valueType.String() {
				return value, true
			}
			// u8[] -> string implicit cast: emit CAST instruction (lowered in separate pass)
			if isU8ArrayType(valueType) && targetType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		}
	case zeus_value.ObjectType:
		switch targetType := targetType.(type) {
		case zeus_value.FunctionType:
			return tc.builder.BuildCoerce(value, targetType, value.GetSpan()), true
		case zeus_value.ArrayType:
			// if the class name is stringifies version of value type, then it is a valid cast
			if valueType.Class.Name == targetType.String() {
				return value, true
			}
			// string -> u8[] implicit cast: emit CAST instruction (lowered in separate pass)
			if valueType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING && isU8ArrayType(targetType) {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		case zeus_value.ObjectType:
			// Upcast: a derived object is assignable to a base reference. Fields are laid out
			// base-first, so the pointer is already a valid base pointer — no cast instruction,
			// just retype the value. Dynamic dispatch still reaches overrides via the object's
			// own vtable pointer.
			if zeus_value.IsSubclassOf(valueType.Class, targetType.Class) {
				return value, true
			}
			// Handle ObjectType -> ObjectType casts (after ToKnownTypesPass converts ArrayType to ObjectType)
			// string -> u8[] (as ObjectType) implicit cast: emit CAST instruction (lowered in separate pass)
			if valueType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING && targetType.Class.Name == "u8[]" {
				u8ArrayType := zeus_value.ArrayType{
					ElementType: zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: value.GetSpan()},
					Span:        value.GetSpan(),
				}
				return tc.builder.BuildCast(value, u8ArrayType, value.GetSpan()), true
			}
			// u8[] (as ObjectType) -> string implicit cast: emit CAST instruction (lowered in separate pass)
			if valueType.Class.Name == "u8[]" && targetType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
				return tc.builder.BuildCast(value, targetType, value.GetSpan()), true
			}
		}
	case zeus_value.NullType:
		switch targetType := targetType.(type) {
		case zeus_value.ObjectType:
			// Promote the null constant to the target object type so the codegen
			// can emit a ConstPointerNull of the correct address-space pointer type.
			return zeus_value.NewConstant(zeus_value.NULL_CONSTANT_VALUE, targetType, value.GetSpan()), true
		}
	}

	return value, false
}

// isStringType checks if a type is a string type
func isStringType(valueType zeus_value.ValueType) bool {
	if objType, ok := valueType.(zeus_value.ObjectType); ok {
		return objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING
	}
	return false
}

// isU8ArrayType checks if a type is u8[] (array of unsigned 8-bit integers)
func isU8ArrayType(valueType zeus_value.ValueType) bool {
	arrayType, ok := valueType.(zeus_value.ArrayType)
	if !ok {
		// Also check ObjectType with u8[] class name
		if objType, ok := valueType.(zeus_value.ObjectType); ok {
			return objType.Class.Name == "u8[]"
		}
		return false
	}
	intType, ok := arrayType.ElementType.(zeus_value.IntType)
	if !ok {
		return false
	}
	return intType.Size == zeus_value.I8 && !intType.Signed
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
			// If one side is a literal that fits the other's type, retype the literal to match
			// rather than promoting the typed side up (keeps `b + 1` at u8, not i32).
			if lc := zeus_value.AsConstant(left); lc != nil && zeus_value.ConstantFitsInIntType(lc.Value, rightValueType) {
				return zeus_value.NewConstant(lc.Value, rightValueType, left.GetSpan()), right
			}
			if rc := zeus_value.AsConstant(right); rc != nil && zeus_value.ConstantFitsInIntType(rc.Value, leftValueType) {
				return left, zeus_value.NewConstant(rc.Value, leftValueType, right.GetSpan())
			}
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

// isNumberInterface reports whether vt is the umbrella `Number` interface.
// isNumberInterface reports whether vt is the primordial `Number` umbrella — by identity, not name,
// so a user-declared `interface Number` that shadows it is not treated as the umbrella.
func isNumberInterface(vt zeus_value.ValueType) bool {
	it := zeus_value.AsInterfaceType(vt)
	return it != nil && it.Interface == zeus_value.Registry.GetInterface(zeus_value.ZEUS_NUMBER_INTERFACE)
}

// emitValueOf emits `value.valueOf()` (the Number umbrella's f64 accessor) before instr; the builder
// insertion point must already be positioned. Returns the f64 result.
//
// This CALL_METHOD is inserted after tcMethodCall would run and is not itself routed through it, but
// it needs no further type checking: it has no arguments, valueOf is a public interface method, and
// its output type is set here (f64). It stays a dynamic call and codegen dispatches it through the
// Number itable like any other interface method call. If a future pass makes tcMethodCall a
// precondition for CALL_METHOD, this insertion site must be revisited.
func (p *TypeCheckingPass) emitValueOf(tc *TypeChecker, value zeus_value.Value, span *token.Span) zeus_value.Value {
	return tc.builder.BuildMethodCall(value, "valueOf", []zeus_value.Value{}, zeus_value.FloatType{Size: zeus_value.F64, Span: span}, nil, span)
}

// emitToString converts value to a `string` by calling toString(), autoboxing a primitive first (its
// box carries toString). Returns (converted, true) when value's type implements Stringify (has
// `toString(): string`, checked structurally), else (value, false). The builder insertion point must
// already be positioned before the consuming instruction.
func (p *TypeCheckingPass) emitToString(tc *TypeChecker, value zeus_value.Value, valueType zeus_value.ValueType, span *token.Span) (zeus_value.Value, bool) {
	stringify := zeus_value.Registry.GetInterface(zeus_value.ZEUS_STRINGIFY_INTERFACE)
	stringClass := zeus_value.Registry.GetClass(zeus_value.ZEUS_PRIMORDIAL_STRING)
	if stringify == nil || stringClass == nil {
		return value, false
	}
	// A primitive is stringified through its box (I32/Bool/...); an object/interface directly.
	receiverType := valueType
	boxClass := boxClassForPrimitive(valueType)
	if boxClass != nil {
		receiverType = zeus_value.NewObjectType(boxClass)
	}
	if !zeus_value.CmpValueType(zeus_value.NewInterfaceType(stringify), receiverType) {
		return value, false
	}
	receiver := value
	if boxClass != nil {
		receiver = p.emitBox(tc, value, valueType, boxClass, span)
	}
	stringType := zeus_value.NewObjectType(stringClass)
	return tc.builder.BuildMethodCall(receiver, "toString", []zeus_value.Value{}, stringType, nil, span), true
}

// maybeUnboxOperand unboxes a boxed binary-operand to its scalar so operators compute on scalars
// (n + 1, n < m, b == c → value comparison). A `Number` (umbrella) unboxes to f64 via valueOf(); a
// concrete box / Bool reads its exact `value` field. Non-box operands pass through unchanged.
func (p *TypeCheckingPass) maybeUnboxOperand(tc *TypeChecker, instr *Instr, value zeus_value.Value) zeus_value.Value {
	vt := tc.getValueType(value)
	if isNumberInterface(vt) {
		tc.builder.SetInsertionBeforeInstr(tc.currentBlock, instr)
		return p.emitValueOf(tc, value, value.GetSpan())
	}
	if objType := zeus_value.AsObjectType(vt); objType != nil && isBoxedPrimordial(objType.Class) {
		// isBoxedPrimordial guarantees the primordial box has a `value` field, but guard anyway so a
		// nil field type never reaches BuildUnbox as an untyped instruction.
		if fieldType := boxFieldType(objType.Class); fieldType != nil {
			tc.builder.SetInsertionBeforeInstr(tc.currentBlock, instr)
			return tc.builder.BuildUnbox(value, fieldType, value.GetSpan())
		}
	}
	return value
}

// coerceStringConcatOperands converts the non-string operand of a `string + X` (or `X + string`) to
// string so it concatenates (`"n=" + 5`, template `${n}`). When both operands are strings (normal
// concat) or neither is (numeric add), operands pass through unchanged. Non-Stringify operands stay
// as-is and fail the operator type check below cleanly.
func (p *TypeCheckingPass) coerceStringConcatOperands(tc *TypeChecker, instr *Instr, left, right zeus_value.Value) (zeus_value.Value, zeus_value.Value) {
	leftIsStr := isStringType(tc.getValueType(left))
	rightIsStr := isStringType(tc.getValueType(right))
	if leftIsStr == rightIsStr {
		return left, right
	}
	stringClass := zeus_value.Registry.GetClass(zeus_value.ZEUS_PRIMORDIAL_STRING)
	if stringClass == nil {
		return left, right
	}
	stringType := zeus_value.NewObjectType(stringClass)
	if leftIsStr {
		if converted, ok := p.tryImplicitCast(tc, instr, right, stringType); ok {
			right = converted
		}
	} else if converted, ok := p.tryImplicitCast(tc, instr, left, stringType); ok {
		left = converted
	}
	return left, right
}

func (p *TypeCheckingPass) tcBinaryOp(tc *TypeChecker, instr *Instr, resultTypeFn func(a, b zeus_value.ValueType) zeus_value.ValueType, cmpTypeFn func(a, b zeus_value.ValueType) bool) {
	input := AsBinaryOpInstrInput(instr.Input)

	// String concatenation coerces the non-string operand to string (via toString). Done before the
	// unbox step so a boxed operand stringifies directly rather than unboxing to a scalar first.
	if instr.Type == InstrTypeAdd {
		input.Left, input.Right = p.coerceStringConcatOperands(tc, instr, input.Left, input.Right)
	}

	// Arithmetic/comparison on boxed primitives operates on the unboxed scalar — but never against
	// null. A box is never null, so `box == null` stays an identity check; this also keeps the
	// compiler-generated receiver null check (`obj == null` before a method call) valid on boxes.
	if !zeus_value.IsNullType(tc.getValueType(input.Left)) && !zeus_value.IsNullType(tc.getValueType(input.Right)) {
		input.Left = p.maybeUnboxOperand(tc, instr, input.Left)
		input.Right = p.maybeUnboxOperand(tc, instr, input.Right)
	}

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

	// A unary operator on a boxed value computes on the unboxed scalar (`!b`, `-n`, `~n`).
	input.Value = p.maybeUnboxOperand(tc, instr, input.Value)

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

	// A boxed condition (e.g. `if (b)` where b: Bool) computes on the unboxed scalar. Bool unboxes
	// to boolean; a non-bool box unboxes to its scalar and then fails the bool check below cleanly.
	input.Condition = p.maybeUnboxOperand(tc, instr, input.Condition)

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

	if input.Addr.IsConst {
		name := input.Addr.OriginalName
		if name == "" {
			name = input.Addr.Name
		}
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot assign to constant '%s'", name),
			Span:    instr.Span,
		})
	}

	// If the variable has no declared type (e.g. ternary result var), infer from stored value.
	if zeus_value.IsUndefinedType(input.Addr.ValueType) {
		input.Addr.ValueType = tc.getValueType(input.Value)
		return
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
			Message: fmt.Sprintf("function must return a value of type '%s'", returnType),
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
	if functionType.IsVariadic {
		return p.tcVariadicFunctionCall(tc, instr, functionType, args, calleeSpan)
	}

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

// tcVariadicFunctionCall type-checks a call to a variadic function. The fixed leading
// parameters are checked positionally; every trailing argument is checked against the
// rest parameter's array element type. Arguments are returned unchanged in shape — the
// VariadicCallLoweringPass later collapses the trailing arguments into an array.
func (p *TypeCheckingPass) tcVariadicFunctionCall(tc *TypeChecker, instr *Instr, functionType zeus_value.FunctionType, args []zeus_value.Value, calleeSpan *token.Span) []zeus_value.Value {
	fixedCount := len(functionType.ParamTypes) - 1

	if len(args) < fixedCount {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected at least %d arguments for function, but found %d", fixedCount, len(args)),
			Span:    calleeSpan,
		})
		return args
	}

	elementType := tc.variadicElementType(functionType.ParamTypes[fixedCount])
	// elementType is nil only when the rest parameter is not an array — that error is
	// already reported during IR gen (checkVariadicParamType), so skip the trailing-arg
	// checks to avoid cascading "<nil>" type errors.
	checkTrailing := elementType != nil

	for i := range args {
		expected := elementType
		if i < fixedCount {
			expected = functionType.ParamTypes[i]
		} else if !checkTrailing {
			continue
		}
		castedArg, ok := p.tryImplicitCast(tc, instr, args[i], expected)
		args[i] = castedArg
		if !ok {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("argument %d of type '%s' does not match expected type '%s'", i+1, tc.getValueType(args[i]), expected),
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

func (p *TypeCheckingPass) tcDeclClass(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassInstrInput(instr.Input)
	class := input.Class

	for _, acc := range class.Accessors {
		// Accessor name must not clash with a data property
		for _, prop := range class.Properties {
			if prop.Property.Name == acc.Name {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("accessor '%s' conflicts with a data property of the same name in class '%s'", acc.Name, class.SourceName()),
					Span:    class.Span,
				})
			}
		}

		if acc.Getter != nil {
			if len(acc.Getter.Params) != 0 {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("getter '%s' must have no parameters", acc.Name),
					Span:    class.Span,
				})
			}
			if zeus_value.IsVoidType(acc.Getter.ReturnType) {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("getter '%s' must have a non-void return type", acc.Name),
					Span:    class.Span,
				})
			}
		}

		if acc.Setter != nil {
			if len(acc.Setter.Params) != 1 {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("setter '%s' must have exactly one parameter", acc.Name),
					Span:    class.Span,
				})
			}
		}

		if acc.Getter != nil && acc.Setter != nil && len(acc.Setter.Params) == 1 {
			if !zeus_value.CmpValueType(acc.Getter.ReturnType, acc.Setter.Params[0].ValueType) {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("getter and setter for '%s' have incompatible types: getter returns '%s', setter expects '%s'", acc.Name, acc.Getter.ReturnType, acc.Setter.Params[0].ValueType),
					Span:    class.Span,
				})
			}
		}
	}
}

// resolveAccessor resolves the object type and finds the named accessor.
// Pushes an error and returns nil if the object is not a class instance or the accessor doesn't exist.
// Also pushes an error (but still returns the accessor) when the accessor is private in the wrong scope.
func (p *TypeCheckingPass) resolveAccessor(tc *TypeChecker, object zeus_value.Value, accessorName string, verb string, span *token.Span) *zeus_value.ClassAccessor {
	// Static accessor: object is a *Class (already validated at IR gen time)
	if class := zeus_value.AsClass(object); class != nil {
		acc := zeus_value.LookupStaticAccessor(class, accessorName)
		if acc == nil {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("static accessor '%s' not found in class '%s'", accessorName, class.SourceName()),
				Span:    span,
			})
		}
		return acc
	}

	valueType := tc.getValueType(object)
	objType := zeus_value.AsObjectType(valueType)
	if objType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot %s property '%s' on non-object type '%s'", verb, accessorName, valueType),
			Span:    span,
		})
		return nil
	}
	acc := zeus_value.LookupAccessor(objType.Class, accessorName)
	if acc == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("property '%s' not found in class '%s'", accessorName, objType.Class.SourceName()),
			Span:    span,
		})
		return nil
	}
	propertyOfSameClass := tc.currentClass != nil && tc.currentClass.Name == objType.Class.Name
	if acc.AccessModifier != nil && acc.AccessModifier.Type != token.TokenTypePublic && !propertyOfSameClass &&
		!tc.protectedAccessAllowed(acc.AccessModifier, objType.Class) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("accessor '%s' is not accessible in class '%s'", accessorName, objType.Class.SourceName()),
			Span:    span,
		})
	}
	return acc
}

func (p *TypeCheckingPass) tcGetAccessor(tc *TypeChecker, instr *Instr) {
	input := AsGetAccessorInstrInput(instr.Input)
	acc := p.resolveAccessor(tc, input.Object, input.AccessorName, "access", instr.Output.Span)
	if acc == nil {
		return
	}
	if acc.Getter == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("property '%s' is write-only (no getter defined)", input.AccessorName),
			Span:    instr.Output.Span,
		})
		return
	}
	acc.Getter.IsUsed = true
	instr.Output.ValueType = acc.Getter.ReturnType
}

func (p *TypeCheckingPass) tcSetAccessor(tc *TypeChecker, instr *Instr) {
	input := AsSetAccessorInstrInput(instr.Input)
	acc := p.resolveAccessor(tc, input.Object, input.AccessorName, "set", instr.Output.Span)
	if acc == nil {
		return
	}
	if acc.Setter == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("property '%s' is read-only (no setter defined)", input.AccessorName),
			Span:    instr.Output.Span,
		})
		return
	}
	// Implicit cast: if value type differs from setter param type, insert a cast.
	if len(acc.Setter.Params) == 1 {
		expectedType := acc.Setter.Params[0].ValueType
		castedValue, ok := p.tryImplicitCast(tc, instr, input.Value, expectedType)
		if !ok {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("cannot assign value of type '%s' to accessor '%s' which expects '%s'", tc.getValueType(input.Value), input.AccessorName, expectedType),
				Span:    instr.Output.Span,
			})
		}
		input.Value = castedValue
		instr.Input = NewSetAccessorInstrInput(input.Object, input.AccessorName, input.Value)
	}
	acc.Setter.IsUsed = true
	instr.Output.ValueType = tc.getValueType(input.Value)
}

func (p *TypeCheckingPass) tcNewObj(tc *TypeChecker, instr *Instr) {
	input := AsNewObjInstrInput(instr.Input)

	if !zeus_value.IsClass(input.Callee) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot create object of type '%s'", tc.getValueType(input.Callee)),
			Span:    input.Callee.GetSpan(),
		})
		// Callee is not a class (e.g. `new (0)`). Stop here rather than building an
		// ObjectType with a nil class, which later type-check stages would dereference.
		return
	}

	class := zeus_value.AsClass(input.Callee)

	var constructorMethod *zeus_value.Function = nil
	var constructorAccessModifier token.TokenType = token.TokenTypePublic

	// Resolve the effective constructor: the class's own, or the nearest inherited one so a
	// derived class without its own constructor forwards to (and is `new`-ed with) the base's.
	if ctorMethod := zeus_value.LookupMethod(class, token.CONSTRUCTOR_METHOD_NAME); ctorMethod != nil {
		constructorMethod = ctorMethod.Method
		if ctorMethod.AccessModifier != nil {
			constructorAccessModifier = ctorMethod.AccessModifier.Type
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

	instr.Output.ValueType = zeus_value.NewObjectType(class)
}

func (p *TypeCheckingPass) tcObjectPropertyAccess(tc *TypeChecker, instr *Instr) {
	input := AsObjectPropertyAccessInstrInput(instr.Input)
	output := instr.Output
	valueType := tc.getValueType(input.Object)

	if zeus_value.IsObjectType(valueType) {
		class := zeus_value.AsObjectType(valueType).Class
		// Inherited members are resolvable: base-first flattened views let a derived class see
		// (and, for same-named members, shadow) its base's fields and methods.
		properties := class.Layout().Fields
		methods := zeus_value.FlattenedMethods(class)
		isFound := false
		isAccessible := false
		isMethod := false
		var foundAccessModifier *token.Token

		// Reject access to static properties on an instance — must use ClassName.x.
		// Walk the parent chain so inherited statics are caught with a targeted message.
		for cur := class; cur != nil; cur = cur.ParentClass {
			for _, prop := range cur.Properties {
				if prop.IsStatic && prop.Property.Name == input.Property {
					tc.pushError(&zeus_error.ZeusError{
						Message: fmt.Sprintf("'%s' is a static property of '%s'; access it as '%s.%s'", input.Property, cur.SourceName(), cur.SourceName(), input.Property),
						Span:    output.Span,
					})
					return
				}
			}
		}

		for _, property := range properties {
			if property.Property.Name == input.Property {
				isFound = true
				foundAccessModifier = property.AccessModifier
				if property.AccessModifier != nil {
					isAccessible = property.AccessModifier.Type == token.TokenTypePublic
				} else {
					isAccessible = false
				}
				instr.Output.ValueType = property.Property.ValueType
				if property.IsReadonly && input.IsLValue {
					isInConstructor := tc.currentFunction != nil &&
						tc.currentFunction.OriginalName == token.CONSTRUCTOR_METHOD_NAME &&
						tc.currentClass != nil &&
						tc.currentClass.Name == class.Name
					if !isInConstructor {
						tc.pushError(&zeus_error.ZeusError{
							Message: fmt.Sprintf("cannot assign to readonly property '%s'", input.Property),
							Span:    output.Span,
						})
					}
				}
			}
		}

		for _, method := range methods {
			if method.Method.SourceName() == input.Property {
				isFound = true
				isMethod = true
				foundAccessModifier = method.AccessModifier
				if method.AccessModifier != nil {
					isAccessible = method.AccessModifier.Type == token.TokenTypePublic
				}
				instr.Output.ValueType = zeus_value.ToFunctionType(*method.Method)
			}
		}

		if !isFound {
			// Special case: provide a clearer error for string immutability
			if class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING && input.Property == zeus_value.ARRAY_METHOD_SET {
				tc.pushError(&zeus_error.ZeusError{
					Message: "cannot assign to string index: strings are immutable",
					Span:    output.Span,
				})
			} else {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("property %s not found in class %s", input.Property, class.SourceName()),
					Span:    output.Span,
				})
			}
		}

		if isFound && isMethod {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("class methods cannot be used as function values; wrap in a fat arrow: (args): ReturnType => { return obj.%s(args); }", input.Property),
				Span:    output.Span,
			})
		} else if isFound {
			propertyOfSameClass := tc.currentClass != nil && tc.currentClass.Name == class.Name
			if !isAccessible && !propertyOfSameClass && !tc.protectedAccessAllowed(foundAccessModifier, class) {
				tc.pushError(&zeus_error.ZeusError{
					Message: fmt.Sprintf("property %s is not accessible in class %s", input.Property, class.SourceName()),
					Span:    output.Span,
				})
			}
		}
	} else if zeus_value.IsInterfaceType(valueType) {
		iface := zeus_value.AsInterfaceType(valueType).Interface
		var found *zeus_value.ClassProperty
		for _, prop := range zeus_value.InterfaceProperties(iface) {
			if prop.Property.Name == input.Property {
				found = prop
				break
			}
		}
		if found == nil {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("interface '%s' has no property '%s'", iface.Name, input.Property),
				Span:    output.Span,
			})
			return
		}
		if found.IsReadonly && input.IsLValue {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("cannot assign to readonly property '%s'", input.Property),
				Span:    output.Span,
			})
		}
		instr.Output.ValueType = found.Property.ValueType
	} else {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot access property %s of type '%s'", input.Property, valueType),
			Span:    output.Span,
		})
	}
}

func (p *TypeCheckingPass) tcMethodCall(tc *TypeChecker, instr *Instr) {
	input := AsMethodCallInstrInput(instr.Input)

	// Skip if output type is already set (lowering-pass emitted instructions)
	if instr.Output.ValueType != nil {
		return
	}

	valueType := tc.getValueType(input.Object)

	// Method call through an interface value: resolve against the interface's method
	// signatures. Codegen dispatches dynamically to the concrete class via an itable.
	if zeus_value.IsInterfaceType(valueType) {
		iface := zeus_value.AsInterfaceType(valueType).Interface
		var ifaceMethod *zeus_value.Function
		for _, m := range zeus_value.InterfaceMethods(iface) {
			if m.SourceName() == input.MethodName {
				ifaceMethod = m
				break
			}
		}
		if ifaceMethod == nil {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("interface '%s' has no method '%s'", iface.Name, input.MethodName),
				Span:    instr.Output.Span,
			})
			return
		}
		functionType := zeus_value.ToFunctionType(*ifaceMethod)
		input.Args = p.tcFunctionCall(tc, instr, functionType, input.Args, instr.Output.Span)
		instr.Input = NewMethodCallInstrInput(input.Object, input.MethodName, input.Args)
		instr.Output.ValueType = ifaceMethod.ReturnType
		return
	}

	if !zeus_value.IsObjectType(valueType) {
		// Autobox a primitive receiver so a method can be called on it: `(5).toString()`,
		// `x.toString()`. Non-boxable receivers (e.g. void) remain an error.
		boxClass := boxClassForPrimitive(valueType)
		if boxClass == nil {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("cannot call method %s on non-object type '%s'", input.MethodName, valueType),
				Span:    instr.Output.Span,
			})
			return
		}
		tc.builder.SetInsertionBeforeInstr(tc.currentBlock, instr)
		// input is the live *MethodCallInstrInput, so mutating Object updates instr.Input in place.
		input.Object = p.emitBox(tc, input.Object, valueType, boxClass, instr.Output.Span)
		valueType = zeus_value.NewObjectType(boxClass)
	}

	class := zeus_value.AsObjectType(valueType).Class
	// super.method() resolves non-virtually on the base class, not the receiver's dynamic class.
	if input.StaticClass != nil {
		class = input.StaticClass
	}
	// Walk the inheritance chain so an inherited (or overridden) method is found; a derived
	// method shadows a same-named base method.
	foundMethod := zeus_value.LookupMethod(class, input.MethodName)

	// Static method called on instance: reject with a targeted error.
	if foundMethod != nil && foundMethod.IsStatic {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("'%s' is a static method of '%s'; call it as '%s.%s(...)'", input.MethodName, class.SourceName(), class.SourceName(), input.MethodName),
			Span:    instr.Output.Span,
		})
		return
	}

	if foundMethod == nil {
		// Check if it is a function-type property being called directly (obj.fnProp(args)).
		// The IR generator cannot distinguish methods from function-type properties at gen time,
		// so it always emits CALL_METHOD for obj.x(args). The lowering pass rewrites these
		// to OBJ_PROP_ACCESS + INDIRECT_CALL after type checking.
		if property := zeus_value.LookupInstanceProperty(class, input.MethodName); property != nil {
			if ft, ok := property.Property.ValueType.(zeus_value.FunctionType); ok {
				// Type-check the arguments against the property's function type
				// (arity + variadic element types + implicit casts), same as a
				// direct/indirect call, before the lowering pass rewrites this.
				input.Args = p.tcFunctionCall(tc, instr, ft, input.Args, instr.Output.Span)
				instr.Input = NewStaticMethodCallInstrInput(input.Object, input.MethodName, input.Args, input.StaticClass)
				instr.Output.ValueType = ft.ReturnType
				return
			}
		}
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("property %s not found in class %s", input.MethodName, class.SourceName()),
			Span:    instr.Output.Span,
		})
		return
	}

	if input.MethodName == token.CONSTRUCTOR_METHOD_NAME {
		tc.pushError(&zeus_error.ZeusError{
			Message: "cannot access constructor method of a class",
			Span:    instr.Output.Span,
		})
		return
	}

	propertyOfSameClass := tc.currentClass != nil && tc.currentClass.Name == class.Name
	if foundMethod.AccessModifier != nil && foundMethod.AccessModifier.Type != token.TokenTypePublic && !propertyOfSameClass &&
		!tc.protectedAccessAllowed(foundMethod.AccessModifier, class) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("property %s is not accessible in class %s", input.MethodName, class.SourceName()),
			Span:    instr.Output.Span,
		})
	}

	functionType := zeus_value.ToFunctionType(*foundMethod.Method)
	input.Args = p.tcFunctionCall(tc, instr, functionType, input.Args, instr.Output.Span)
	instr.Input = NewStaticMethodCallInstrInput(input.Object, input.MethodName, input.Args, input.StaticClass)

	instr.Output.ValueType = foundMethod.Method.ReturnType
}

// tcSuperConstructorCall type-checks super(...) arguments against the base constructor's
// parameters. ParentClass is the nearest ancestor that declares a constructor (set at IR gen).
func (p *TypeCheckingPass) tcSuperConstructorCall(tc *TypeChecker, instr *Instr) {
	input := AsSuperConstructorCallInstrInput(instr.Input)

	var constructor *zeus_value.Function
	for _, method := range input.ParentClass.Methods {
		if method.Method.SourceName() == token.CONSTRUCTOR_METHOD_NAME {
			constructor = method.Method
			break
		}
	}
	if constructor == nil {
		return // IR gen guarantees ParentClass has a constructor; nothing to check otherwise
	}

	if len(input.Args) != len(constructor.Params) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("expected %d arguments for super(...), but found %d", len(constructor.Params), len(input.Args)),
			Span:    instr.Output.Span,
		})
		return
	}
	for i := range input.Args {
		input.Args[i] = p.cmpValueWithImplicitCast(tc, instr, constructor.Params[i].ValueType, input.Args[i])
	}
	instr.Input = NewSuperConstructorCallInstrInput(input.ParentClass, input.ThisObject, input.Args)
}

func (p *TypeCheckingPass) tcGetIndex(tc *TypeChecker, instr *Instr) {
	input := AsGetIndexInstrInput(instr.Input)
	output := instr.Output
	targetType := tc.getValueType(input.Array)

	// Check that the value being indexed is an object type (array or string)
	if !zeus_value.IsObjectType(targetType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot use indexing operator [] on type '%s', expected an array or string", targetType),
			Span:    instr.Span,
		})
		return
	}

	objType := zeus_value.AsObjectType(targetType)

	// Check if this is a string (string indexing returns u8)
	if objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
		// String indexing only supports a single index
		if len(input.Indices) != 1 {
			tc.pushError(&zeus_error.ZeusError{
				Message: "string indexing only supports a single index",
				Span:    instr.Span,
			})
			return
		}

		// Validate index is an integer type
		indexType := tc.getValueType(input.Indices[0])
		if !zeus_value.IsIntType(indexType) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("string index must be an integer, got '%s'", indexType),
				Span:    instr.Span,
			})
		}

		// String indexing returns u8 (unsigned byte)
		output.ValueType = zeus_value.IntType{Size: zeus_value.I8, Signed: false, Span: instr.Span}
		return
	}

	// Check if this is actually an array class (has ArrayElementType)
	if objType.Class.ArrayElementType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot use indexing operator [] on type '%s', expected an array or string", objType.Class.Name),
			Span:    instr.Span,
		})
		return
	}

	// Validate each index is an integer type
	for _, index := range input.Indices {
		indexType := tc.getValueType(index)
		if !zeus_value.IsIntType(indexType) {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("array index must be an integer, got '%s'", indexType),
				Span:    instr.Span,
			})
		}
	}

	// Determine the result type by peeling off array dimensions
	// For array[i][j], we need to peel off one dimension per index
	resultType := targetType
	for range input.Indices {
		if objType := zeus_value.AsObjectType(resultType); objType != nil && objType.Class.ArrayElementType != nil {
			resultType = objType.Class.ArrayElementType
		} else {
			tc.pushError(&zeus_error.ZeusError{
				Message: fmt.Sprintf("too many indices for array type '%s'", targetType),
				Span:    instr.Span,
			})
			return
		}
	}

	// If the result is still an array type, wrap it in ObjectType
	if arrayResultType, ok := resultType.(zeus_value.ArrayType); ok {
		resultClass := tc.getClassFromArrayType(arrayResultType)
		resultType = zeus_value.NewObjectType(resultClass)
	}

	output.ValueType = resultType
}

func (p *TypeCheckingPass) tcSetIndex(tc *TypeChecker, instr *Instr) {
	input := AsSetIndexInstrInput(instr.Input)
	targetType := tc.getValueType(input.Array)

	if !zeus_value.IsObjectType(targetType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot use indexing operator [] on type '%s', expected an array", targetType),
			Span:    instr.Span,
		})
		return
	}

	objType := zeus_value.AsObjectType(targetType)

	if objType.Class.Name == zeus_value.ZEUS_PRIMORDIAL_STRING {
		tc.pushError(&zeus_error.ZeusError{
			Message: "strings are immutable and cannot be assigned via indexing",
			Span:    instr.Span,
		})
		return
	}

	if objType.Class.ArrayElementType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("cannot use indexing operator [] on type '%s', expected an array", objType.Class.Name),
			Span:    instr.Span,
		})
		return
	}

	indexType := tc.getValueType(input.Index)
	if !zeus_value.IsIntType(indexType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("array index must be an integer, got '%s'", indexType),
			Span:    instr.Span,
		})
	}
}

func (p *TypeCheckingPass) tcIndirectFuncCall(tc *TypeChecker, instr *Instr) {
	input := AsIndirectFuncCallInstrInput(instr.Input)

	methodType := tc.getValueType(input.Function)
	functionType := zeus_value.AsFunctionType(methodType)
	objectType := zeus_value.AsObjectType(methodType)
	fnSpan := input.Function.GetSpan()

	// TC runs before lowering, so functor INDIRECT_FUNC_CALLs haven't been rewritten to CALL_METHOD yet.
	if objectType != nil {
		for _, method := range objectType.Class.Methods {
			// A nil access modifier means the default (public).
			isPublic := method.AccessModifier == nil || method.AccessModifier.Type == token.TokenTypePublic
			if method.Method.SourceName() == token.FUNCTOR_CALL_METHOD_NAME && isPublic {
				functionType = zeus_value.AsFunctionType(zeus_value.GetValueType(method.Method))
				method.Method.IsUsed = true
				fnSpan = method.Method.GetSpan()
				break
			}
		}
	}

	if functionType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: "expression is not callable",
			Span:    instr.Output.Span,
		})
		return
	}

	input.Args = p.tcFunctionCall(tc, instr, *functionType, input.Args, fnSpan)
	instr.Input = NewIndirectFuncCallInstrInput(input.Function, input.Args)

	instr.Output.ValueType = functionType.ReturnType
}

func (p *TypeCheckingPass) tcDeclClassMethod(tc *TypeChecker, instr *Instr) {
	input := AsDeclClassMethodInstrInput(instr.Input)
	p.validateFunctionReturns(tc, input.Method, input.Body)
}

// tcThrow validates that the thrown expression is an Error class or subclass
func (p *TypeCheckingPass) tcThrow(tc *TypeChecker, instr *Instr) {
	input := AsThrowInstrInput(instr.Input)
	objectPtr := input.ObjectPtr

	// Get the type of the thrown value
	valueType := tc.getValueType(objectPtr)

	// Check if it's an object type
	if !zeus_value.IsObjectType(valueType) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("throw expression must be an Error or subclass, but found '%s'", valueType),
			Span:    instr.Span,
		})
		return
	}

	// Check if the class is Error or a subclass of Error
	objectType := zeus_value.AsObjectType(valueType)
	if objectType == nil {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("throw expression must be an Error or subclass, but found '%s'", valueType),
			Span:    instr.Span,
		})
		return
	}

	if !zeus_value.IsErrorClass(objectType.Class) {
		tc.pushError(&zeus_error.ZeusError{
			Message: fmt.Sprintf("throw expression must be an Error or subclass, but found class '%s'", objectType.Class.SourceName()),
			Span:    instr.Span,
		})
	}
}
