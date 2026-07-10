package zeus_value

import (
	"github.com/ameerthehacker/zeus/internal/token"
)

const ZEUS_PRIMORDIAL_ARRAY = "array"
const ZEUS_PRIMORDIAL_STRING = "string"
const ZEUS_PRIMORDIAL_ERROR = "error"

// Reserved class ID for Error - used for exception type matching
const ERROR_CLASS_ID = 1

// Array method names
const (
	ARRAY_METHOD_PUSH               = "push"
	ARRAY_METHOD_POP                = "pop"
	ARRAY_METHOD_GET                = "get"
	ARRAY_METHOD_SET                = "set"
	ARRAY_METHOD_COPY               = "copy"
	ARRAY_METHOD_COPY_RANGE         = "copyRange"
	ARRAY_METHOD_COPY_RANGE_REVERSE = "copyRangeReversed"
	ARRAY_METHOD_CONCAT             = "concat"
	ARRAY_METHOD_SLICE              = "slice"
	ARRAY_METHOD_INDEX_OF           = "indexOf"
	ARRAY_METHOD_LAST_INDEX         = "lastIndexOf"
	ARRAY_METHOD_FIND               = "find"
	ARRAY_METHOD_FIND_INDEX         = "findIndex"
	ARRAY_METHOD_INCLUDES           = "includes"
	ARRAY_METHOD_REVERSE            = "reverse"
	ARRAY_METHOD_FILL               = "fill"
	ARRAY_METHOD_CLEAR              = "clear"
	ARRAY_METHOD_IS_EMPTY           = "isEmpty"
)

// Array property names
const (
	ARRAY_PROPERTY_CAPACITY = "capacity"
	ARRAY_PROPERTY_LENGTH   = "_length"
	ARRAY_PROPERTY_DATA     = "data"
)

// String property names
const (
	STRING_PROPERTY_DATA   = "data"
	STRING_PROPERTY_LENGTH = "length"
)

// String method names
const (
	STRING_METHOD_COMPARE = "compare"
	STRING_METHOD_EQUALS  = "equals"
	STRING_METHOD_CONCAT  = "concat"
)

func GetArrayPrimordialClassDefinition(arrayType ArrayType) *Class {
	span := arrayType.GetSpan()
	// Use i32 (signed) for parameters/properties that interact with user code
	// This allows negative index checking at runtime and -1 return values for search
	i32Type := IntType{Size: I32, Signed: true, Span: span}
	selfArrayType := NewArrayType(arrayType.ElementType, span)

	// capacity of the array (private - internal implementation detail)
	capacityProperty := NewClassProperty(NewVar("capacity", i32Type, false, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	// _length of the array (private - exposed via getter accessor "length")
	lengthProperty := NewClassProperty(NewVar("_length", i32Type, false, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	// opaque pointer to the data of the array (private - internal implementation)
	dataProperty := NewClassProperty(NewVar("data", OpaqueType{Span: span}, true, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	properties := []*ClassProperty{capacityProperty, lengthProperty, dataProperty}

	// Helper function to create a public method
	publicMethod := func(method *Function) *ClassMethod {
		return NewClassMethod(method, &token.Token{Type: token.TokenTypePublic, Span: span})
	}

	// Helper function to create a public method that is lowered (no runtime wrapper needed)
	loweredMethod := func(method *Function) *ClassMethod {
		return NewLoweredClassMethod(method, &token.Token{Type: token.TokenTypePublic, Span: span})
	}

	// Constructor
	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("capacity", i32Type, false, span),
	}, VoidType{Span: span}, span)

	// Basic element operations
	pushMethod := NewFunction(ARRAY_METHOD_PUSH, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, VoidType{Span: span}, span)

	popMethod := NewFunction(ARRAY_METHOD_POP, []*Var{}, arrayType.ElementType, span)

	getMethod := NewFunction(ARRAY_METHOD_GET, []*Var{
		NewVar("index", i32Type, false, span),
	}, arrayType.ElementType, span)

	setMethod := NewFunction(ARRAY_METHOD_SET, []*Var{
		NewVar("index", i32Type, false, span),
		NewVar("value", arrayType.ElementType, false, span),
	}, VoidType{Span: span}, span)

	// Array copy operations
	copyMethod := NewFunction(ARRAY_METHOD_COPY, []*Var{
		NewVar("source", selfArrayType, false, span),
	}, VoidType{Span: span}, span)

	// copyRange: copies a range of elements from source to this array
	// Used internally by lowering for concat/slice
	copyRangeMethod := NewFunction(ARRAY_METHOD_COPY_RANGE, []*Var{
		NewVar("source", selfArrayType, false, span),
		NewVar("srcOffset", i32Type, false, span),
		NewVar("destOffset", i32Type, false, span),
		NewVar("count", i32Type, false, span),
	}, VoidType{Span: span}, span)

	// copyRangeReversed: copies a range of elements from source in reverse order
	// Used internally by lowering for reverse
	copyRangeReversedMethod := NewFunction(ARRAY_METHOD_COPY_RANGE_REVERSE, []*Var{
		NewVar("source", selfArrayType, false, span),
		NewVar("srcOffset", i32Type, false, span),
		NewVar("destOffset", i32Type, false, span),
		NewVar("count", i32Type, false, span),
	}, VoidType{Span: span}, span)

	// concat: returns a new array with elements from both arrays
	concatMethod := NewFunction(ARRAY_METHOD_CONCAT, []*Var{
		NewVar("other", selfArrayType, false, span),
	}, selfArrayType, span)

	// slice: returns a new array with elements from start (inclusive) to end (exclusive)
	sliceMethod := NewFunction(ARRAY_METHOD_SLICE, []*Var{
		NewVar("start", i32Type, false, span),
		NewVar("end", i32Type, false, span),
	}, selfArrayType, span)

	// Search operations
	// indexOf: returns index of first occurrence, -1 if not found
	indexOfMethod := NewFunction(ARRAY_METHOD_INDEX_OF, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, i32Type, span)

	// lastIndexOf: returns index of last occurrence, -1 if not found
	lastIndexOfMethod := NewFunction(ARRAY_METHOD_LAST_INDEX, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, i32Type, span)

	// find: returns the first element that equals value, or default if not found
	findMethod := NewFunction(ARRAY_METHOD_FIND, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, arrayType.ElementType, span)

	// findIndex: returns index of first element that equals value, -1 if not found
	// (same as indexOf, provided for API consistency)
	findIndexMethod := NewFunction(ARRAY_METHOD_FIND_INDEX, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, i32Type, span)

	// includes: returns true if array contains the value
	includesMethod := NewFunction(ARRAY_METHOD_INCLUDES, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, BoolType{Span: span}, span)

	// Non-mutating operations
	// reverse: returns a new array with elements in reverse order
	reverseMethod := NewFunction(ARRAY_METHOD_REVERSE, []*Var{}, selfArrayType, span)

	// fill: fills all elements with the given value
	fillMethod := NewFunction(ARRAY_METHOD_FILL, []*Var{
		NewVar("value", arrayType.ElementType, false, span),
	}, VoidType{Span: span}, span)

	// clear: clears all elements (sets length to 0)
	clearMethod := NewFunction(ARRAY_METHOD_CLEAR, []*Var{}, VoidType{Span: span}, span)

	// State check
	// isEmpty: returns true if length is 0
	isEmptyMethod := NewFunction(ARRAY_METHOD_IS_EMPTY, []*Var{}, BoolType{Span: span}, span)

	methods := []*ClassMethod{
		publicMethod(constructorMethod),
		publicMethod(pushMethod),
		publicMethod(popMethod),
		publicMethod(getMethod),
		publicMethod(setMethod),
		publicMethod(copyMethod),
		publicMethod(copyRangeMethod),
		publicMethod(copyRangeReversedMethod), // Used by reverse lowering
		loweredMethod(concatMethod),           // Handled by IR lowering, no runtime call
		loweredMethod(sliceMethod),            // Handled by IR lowering, no runtime call
		publicMethod(indexOfMethod),
		publicMethod(lastIndexOfMethod),
		publicMethod(findMethod),
		publicMethod(findIndexMethod),
		publicMethod(includesMethod),
		loweredMethod(reverseMethod), // Handled by IR lowering, no runtime call
		publicMethod(fillMethod),
		publicMethod(clearMethod),
		publicMethod(isEmptyMethod),
	}

	// length getter: exposes the private _length field as a read-only accessor.
	// IsLowered=true means AccessorLoweringPass expands it to OBJECT_PROPERTY_ACCESS(_length)+LOAD
	// without needing a Zig runtime function.
	lengthGetterFn := NewFunction("#get_length", []*Var{}, i32Type, span)
	lengthAccessor := NewClassAccessor("length", lengthGetterFn, nil, &token.Token{Type: token.TokenTypePublic, Span: span})
	lengthAccessor.IsLowered = true

	return NewClass(arrayType.String(), properties, methods, []*ClassAccessor{lengthAccessor}, ZEUS_PRIMORDIAL_ARRAY, arrayType.ElementType, span)
}

// string is nothing but an array of u8
// in the runtime constructor we intern the string and return the same pointer
func GetStringPrimordialClassDefinition(span *token.Span) *Class {
	u8ArrayObjectType := ObjectType{Class: GetArrayPrimordialClassDefinition(ArrayType{ElementType: IntType{Size: I8, Signed: false, Span: span}, Span: span})}

	// Properties
	dataProperty := NewClassProperty(NewVar(STRING_PROPERTY_DATA, u8ArrayObjectType, true, span), &token.Token{Type: token.TokenTypePrivate, Span: span})
	lengthProperty := NewClassProperty(NewVar(STRING_PROPERTY_LENGTH, IntType{Size: I32, Signed: true, Span: span}, false, span), &token.Token{Type: token.TokenTypePublic, Span: span})
	properties := []*ClassProperty{dataProperty, lengthProperty}

	// Create the string class first (without methods that reference itself)
	stringClass := NewClass(ZEUS_PRIMORDIAL_STRING, properties, nil, nil, ZEUS_PRIMORDIAL_STRING, nil, span)

	// Methods - use UserDefinedType for self-reference to avoid copy issues
	// The type checker will resolve "string" to the actual string ObjectType
	selfType := UserDefinedType{Name: ZEUS_PRIMORDIAL_STRING, Span: span}

	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("bytes", u8ArrayObjectType, true, span),
	}, VoidType{Span: span}, span)
	// compare: returns -1 if this < other, 0 if equal, 1 if this > other
	compareMethod := NewFunction(STRING_METHOD_COMPARE, []*Var{
		NewVar("other", selfType, true, span),
	}, IntType{Size: I8, Signed: true, Span: span}, span)
	// equals: returns true if strings are equal, false otherwise
	equalsMethod := NewFunction(STRING_METHOD_EQUALS, []*Var{
		NewVar("other", selfType, true, span),
	}, BoolType{Span: span}, span)
	// concat: returns a new string with the other string appended
	concatMethod := NewFunction(STRING_METHOD_CONCAT, []*Var{
		NewVar("other", selfType, true, span),
	}, selfType, span)

	// Update the class with methods
	stringClass.Methods = []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(compareMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(equalsMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
		NewClassMethod(concatMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
	}

	return stringClass
}

// Error property names
const (
	ERROR_PROPERTY_NAME    = "name"
	ERROR_PROPERTY_MESSAGE = "message"
)

// GetErrorPrimordialClassDefinition returns the built-in Error class definition
// Error is the base class for all exception types in Zeus
// It has a reserved class ID (1) for efficient exception type matching at runtime
func GetErrorPrimordialClassDefinition(span *token.Span) *Class {
	// Error class has name and message properties which are strings
	stringType := UserDefinedType{Name: ZEUS_PRIMORDIAL_STRING, Span: span}

	// name property (public) - the error type name (e.g., "Error", "IndexOutOfBoundsException")
	nameProperty := NewClassProperty(
		NewVar(ERROR_PROPERTY_NAME, stringType, true, span),
		&token.Token{Type: token.TokenTypePublic, Span: span},
	)

	// message property (public) - the error message
	messageProperty := NewClassProperty(
		NewVar(ERROR_PROPERTY_MESSAGE, stringType, true, span),
		&token.Token{Type: token.TokenTypePublic, Span: span},
	)
	properties := []*ClassProperty{nameProperty, messageProperty}

	// Constructor takes name and message strings
	constructorMethod := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{
		NewVar("name", stringType, true, span),
		NewVar("msg", stringType, true, span),
	}, VoidType{Span: span}, span)

	methods := []*ClassMethod{
		NewClassMethod(constructorMethod, &token.Token{Type: token.TokenTypePublic, Span: span}),
	}

	// Use reserved class ID for Error class
	return NewClassWithId(ERROR_CLASS_ID, "Error", properties, methods, ZEUS_PRIMORDIAL_ERROR, span)
}

const ZEUS_PRIMORDIAL_CONSOLE = "Console"

// Console method names
const (
	CONSOLE_METHOD_LOG   = "log"
	CONSOLE_METHOD_ERROR = "error"
	CONSOLE_METHOD_INFO  = "info"
)

// GetConsolePrimordialClassDefinition returns the built-in Console class definition.
// It has no properties and no constructor; methods are backed by runtime ABI functions.
func GetConsolePrimordialClassDefinition(span *token.Span) *Class {
	stringType := UserDefinedType{Name: ZEUS_PRIMORDIAL_STRING, Span: span}
	publicMethod := func(fn *Function) *ClassMethod {
		return NewClassMethod(fn, &token.Token{Type: token.TokenTypePublic, Span: span})
	}
	logMethod := NewFunction(CONSOLE_METHOD_LOG, []*Var{
		NewVar("message", stringType, true, span),
	}, VoidType{Span: span}, span)
	errorMethod := NewFunction(CONSOLE_METHOD_ERROR, []*Var{
		NewVar("message", stringType, true, span),
	}, VoidType{Span: span}, span)
	infoMethod := NewFunction(CONSOLE_METHOD_INFO, []*Var{
		NewVar("message", stringType, true, span),
	}, VoidType{Span: span}, span)
	return NewClass(ZEUS_PRIMORDIAL_CONSOLE, []*ClassProperty{}, []*ClassMethod{
		publicMethod(logMethod),
		publicMethod(errorMethod),
		publicMethod(infoMethod),
	}, nil, ZEUS_PRIMORDIAL_CONSOLE, nil, span)
}

// Ref cell class naming constants
const ZEUS_REF_CELL_CLASS_PREFIX = "__ref_cell_"
const ZEUS_REF_CELL_VALUE_PROPERTY = "value"

// RefCellClassName returns the IR-level class name for a ref cell wrapping the given type.
func RefCellClassName(valueType ValueType) string {
	return ZEUS_REF_CELL_CLASS_PREFIX + valueType.String() + "__"
}

// GetRefCellClassDefinition builds the class definition for a ref cell wrapping valueType.
// The class has a single public `value` property and a no-op constructor.
// Ref cells are allocated on-demand by emitFunction/VisitVarDeclStmt for variables that
// escape into nested closures; they are not pre-registered in the primordial registry.
func GetRefCellClassDefinition(valueType ValueType, span *token.Span) *Class {
	valueProp := NewClassProperty(
		NewVar(ZEUS_REF_CELL_VALUE_PROPERTY, valueType, false, span),
		&token.Token{Type: token.TokenTypePublic, Span: span},
	)
	constructorFn := NewFunction(token.CONSTRUCTOR_METHOD_NAME, []*Var{}, VoidType{Span: span}, span)
	methods := []*ClassMethod{
		NewClassMethod(constructorFn, &token.Token{Type: token.TokenTypePublic, Span: span}),
	}
	return NewClass(RefCellClassName(valueType), []*ClassProperty{valueProp}, methods, nil, "", nil, span)
}

// IsErrorClass checks if a class is the Error class or derives from it
func IsErrorClass(class *Class) bool {
	if class == nil {
		return false
	}
	// Check if this is the Error class itself
	if class.Id == ERROR_CLASS_ID {
		return true
	}
	// Check parent class chain
	current := class.ParentClass
	for current != nil {
		if current.Id == ERROR_CLASS_ID {
			return true
		}
		current = current.ParentClass
	}
	return false
}
