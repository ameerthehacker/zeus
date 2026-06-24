package ir_test

import (
	"testing"

	"github.com/ameerthehacker/zeus/internal/ir"
	"github.com/ameerthehacker/zeus/internal/zeus_error"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// generateLIR runs HIR generation, type checking, and lowering, returning the
// fully-lowered IR builder and its instruction list.
func generateLIR(t *testing.T, source string) (*ir.IRBuilder, []*ir.Instr) {
	t.Helper()
	builder, _ := generateHIR(t, source)

	tc := ir.NewTypeChecker(builder, false)
	tcErrors := tc.TypeCheck()
	for _, err := range tcErrors {
		if err.Severity == zeus_error.ErrorSeverityError {
			t.Fatalf("type check error: %v", err)
		}
	}

	lowerer := ir.NewLowerer(builder)
	lowerer.Lower()

	return builder, builder.GetInstrs()
}

// ---- String Operator Lowering Tests ----

// TestStringConcatLowering verifies that the ADD instruction emitted for string +
// at HIR stage is replaced by an OBJECT_PROPERTY_ACCESS("concat") + LOAD + CALL_INDIRECT_FUNC
// sequence during lowering.
func TestStringConcatLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): string {
    return a + b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// After lowering, no plain ADD should remain for string operands
	adds := findInstrs(all, ir.InstrTypeAdd)
	if len(adds) != 0 {
		t.Errorf("expected 0 ADD instructions after string concat lowering, got %d", len(adds))
	}

	// .concat() method access must be emitted
	concats := findPropertyAccess(all, "concat")
	if len(concats) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('concat') after lowering string +")
	}

	// The method must be called via CALL_INDIRECT_FUNC
	calls := findInstrs(all, ir.InstrTypeIndirectFuncCall)
	if len(calls) == 0 {
		t.Error("expected CALL_INDIRECT_FUNC for concat method call")
	}
}

// TestStringEqLowering verifies that EQ_EQ on strings is lowered to an equals() call.
func TestStringEqLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a == b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// No EQ_EQ should remain for string operands
	eqs := findInstrs(all, ir.InstrTypeEqEq)
	if len(eqs) != 0 {
		t.Errorf("expected 0 EQ_EQ instructions after string eq lowering, got %d", len(eqs))
	}

	// .equals() method access must be emitted
	equalsAccess := findPropertyAccess(all, "equals")
	if len(equalsAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('equals') after lowering string ==")
	}

	calls := findInstrs(all, ir.InstrTypeIndirectFuncCall)
	if len(calls) == 0 {
		t.Error("expected CALL_INDIRECT_FUNC for equals() call")
	}
}

// TestStringNotEqLowering verifies that != on strings is lowered to NOT(equals()).
func TestStringNotEqLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a != b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// No raw NOT_EQ should remain
	neqs := findInstrs(all, ir.InstrTypeNotEq)
	if len(neqs) != 0 {
		t.Errorf("expected 0 NOT_EQ instructions after string neq lowering, got %d", len(neqs))
	}

	// .equals() is called and its result is negated with NOT
	equalsAccess := findPropertyAccess(all, "equals")
	if len(equalsAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('equals') for string !=")
	}
	nots := findInstrs(all, ir.InstrTypeNot)
	if len(nots) == 0 {
		t.Error("expected NOT instruction to negate equals() result for string !=")
	}
}

// TestStringLtLowering verifies that < on strings is lowered to compare() < 0.
func TestStringLtLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a < b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// The original LESS_THAN on string objects is replaced, but a new LESS_THAN
	// comparing compare()'s i32 return value against 0 must appear.
	compareAccess := findPropertyAccess(all, "compare")
	if len(compareAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('compare') for string <")
	}
	lts := findInstrs(all, ir.InstrTypeLessThan)
	if len(lts) == 0 {
		t.Error("expected LESS_THAN instruction comparing compare() result to 0")
	}
}

// TestStringLteLowering verifies that <= on strings is lowered to compare() <= 0.
func TestStringLteLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a <= b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	compareAccess := findPropertyAccess(all, "compare")
	if len(compareAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('compare') for string <=")
	}
	ltes := findInstrs(all, ir.InstrTypeLessThanEq)
	if len(ltes) == 0 {
		t.Error("expected LESS_THAN_EQ instruction for string <=")
	}
}

// TestStringGtLowering verifies that > on strings is lowered to compare() > 0.
func TestStringGtLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a > b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	compareAccess := findPropertyAccess(all, "compare")
	if len(compareAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('compare') for string >")
	}
	gts := findInstrs(all, ir.InstrTypeGreaterThan)
	if len(gts) == 0 {
		t.Error("expected GREATER_THAN instruction for string >")
	}
}

// TestStringGteLowering verifies that >= on strings is lowered to compare() >= 0.
func TestStringGteLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(a: string, b: string): boolean {
    return a >= b
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)
	compareAccess := findPropertyAccess(all, "compare")
	if len(compareAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('compare') for string >=")
	}
	gtes := findInstrs(all, ir.InstrTypeGreaterThanEq)
	if len(gtes) == 0 {
		t.Error("expected GREATER_THAN_EQ instruction for string >=")
	}
}

// ---- Array Index Lowering Tests ----

// TestArrayIndexLowering verifies that arr[i] (GET_INDEX in HIR) is lowered to arr.get(i).
func TestArrayIndexLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(arr: i32[]): i32 {
    return arr[0]
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// No GET_INDEX should remain after lowering
	gets := findInstrs(all, ir.InstrTypeGetIndex)
	if len(gets) != 0 {
		t.Errorf("expected 0 GET_INDEX instructions after lowering, got %d", len(gets))
	}

	// .get() method access must be emitted
	getAccess := findPropertyAccess(all, "get")
	if len(getAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('get') for lowered array indexing")
	}

	calls := findInstrs(all, ir.InstrTypeIndirectFuncCall)
	if len(calls) == 0 {
		t.Error("expected CALL_INDIRECT_FUNC for .get() call")
	}
}

// TestNestedArrayIndexLowering verifies that arr[i][j] produces two .get() call sequences.
func TestNestedArrayIndexLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(arr: i32[][]): i32 {
    return arr[1][2]
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// All GET_INDEX instructions must be gone
	gets := findInstrs(all, ir.InstrTypeGetIndex)
	if len(gets) != 0 {
		t.Errorf("expected 0 GET_INDEX after lowering nested index, got %d", len(gets))
	}

	// Two separate .get() accesses should be emitted (one per dimension)
	getAccess := findPropertyAccess(all, "get")
	if len(getAccess) < 2 {
		t.Errorf("expected at least 2 OBJECT_PROPERTY_ACCESS('get') for arr[i][j], got %d", len(getAccess))
	}

	calls := findInstrs(all, ir.InstrTypeIndirectFuncCall)
	if len(calls) < 2 {
		t.Errorf("expected at least 2 CALL_INDIRECT_FUNC for nested .get() calls, got %d", len(calls))
	}
}

// ---- String/Array Cast Lowering Tests ----

// TestStringToU8ArrayCastLowering verifies that implicit string → u8[] coercion (inserted by
// the type checker) is lowered to a sequence that copies the string's internal data array.
func TestStringToU8ArrayCastLowering(t *testing.T) {
	// The type checker inserts a CAST when assigning a string to a u8[] variable.
	// The lowering pass replaces that CAST with explicit array construction and copy.
	builder, _ := generateLIR(t, `
function test(s: string): u8[] {
    let bytes: u8[] = s
    return bytes
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// No CAST instruction should remain
	casts := findInstrs(all, ir.InstrTypeCast)
	if len(casts) != 0 {
		t.Errorf("expected 0 CAST instructions after string→u8[] lowering, got %d", len(casts))
	}

	// A new u8[] object must be created
	newObjs := findInstrs(all, ir.InstrTypeNewObj)
	if len(newObjs) == 0 {
		t.Error("expected NEW_OBJ for u8[] allocation during string→u8[] lowering")
	}

	// The data must be copied via a .copy() method call
	copyAccess := findPropertyAccess(all, "copy")
	if len(copyAccess) == 0 {
		t.Error("expected OBJECT_PROPERTY_ACCESS('copy') for data copy during string→u8[] lowering")
	}
}

// TestNullAssignment verifies that assigning null to an object variable passes
// type checking and that the stored value is an ObjectType constant (not raw NullType),
// so the codegen can emit a correctly-typed null pointer.
func TestNullAssignment(t *testing.T) {
	builder, _ := generateLIR(t, `
class Node {
  public value: i32;
}
function test(): void {
  let n: Node = new Node()
  n = null
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	stores := findInstrs(all, ir.InstrTypeStore)
	var nullStore *ir.Instr
	for _, s := range stores {
		input := ir.AsStoreInstrInput(s.Input)
		if c := zeus_value.AsConstant(input.Value); c != nil && c.Value == "null" {
			nullStore = s
			break
		}
	}
	if nullStore == nil {
		t.Fatal("expected a STORE instruction with null value")
	}
	input := ir.AsStoreInstrInput(nullStore.Input)
	c := zeus_value.AsConstant(input.Value)
	if _, ok := c.ValueType.(zeus_value.ObjectType); !ok {
		t.Errorf("expected null constant to have ObjectType after TC, got %T", c.ValueType)
	}
}

// TestNullDeclaration verifies that initialising an object variable with null
// passes type checking and the initializer constant is promoted to ObjectType.
func TestNullDeclaration(t *testing.T) {
	builder, _ := generateLIR(t, `
class Node {
  public value: i32;
}
function test(): void {
  let n: Node = null
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	decls := findInstrs(all, ir.InstrTypeDeclVar)
	var nullDecl *ir.Instr
	for _, d := range decls {
		input := ir.AsDeclVarInstrInput(d.Input)
		if c := zeus_value.AsConstant(input.Initializer); c != nil && c.Value == "null" {
			nullDecl = d
			break
		}
	}
	if nullDecl == nil {
		t.Fatal("expected a DECLARE_VAR with null initializer")
	}
	input := ir.AsDeclVarInstrInput(nullDecl.Input)
	c := zeus_value.AsConstant(input.Initializer)
	if _, ok := c.ValueType.(zeus_value.ObjectType); !ok {
		t.Errorf("expected null initializer to have ObjectType after TC, got %T", c.ValueType)
	}
}

// TestNullReturn verifies that returning null from an object-returning function
// passes type checking and the return value constant is promoted to ObjectType.
func TestNullReturn(t *testing.T) {
	builder, _ := generateLIR(t, `
class Node {
  public value: i32;
}
function test(): Node {
  return null
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	returns := findInstrs(all, ir.InstrTypeReturn)
	if len(returns) == 0 {
		t.Fatal("expected a RETURN instruction")
	}
	input := ir.AsReturnInstrInput(returns[0].Input)
	c := zeus_value.AsConstant(input.Value)
	if c == nil || c.Value != "null" {
		t.Fatal("expected return value to be null constant")
	}
	if _, ok := c.ValueType.(zeus_value.ObjectType); !ok {
		t.Errorf("expected null return value to have ObjectType after TC, got %T", c.ValueType)
	}
}

// TestU8ArrayToStringCastLowering verifies that implicit u8[] → string coercion is lowered
// to a sequence that creates a new string from the byte array.
func TestU8ArrayToStringCastLowering(t *testing.T) {
	builder, _ := generateLIR(t, `
function test(b: u8[]): string {
    let s: string = b
    return s
}`)
	entry := getFuncEntryBlock(builder, "test")
	if entry == nil {
		t.Fatal("function 'test' not found")
	}
	all := allBlockInstrs(entry)

	// No CAST instruction should remain
	casts := findInstrs(all, ir.InstrTypeCast)
	if len(casts) != 0 {
		t.Errorf("expected 0 CAST instructions after u8[]→string lowering, got %d", len(casts))
	}

	// A new string object must be created
	newObjs := findInstrs(all, ir.InstrTypeNewObj)
	if len(newObjs) == 0 {
		t.Error("expected NEW_OBJ for string allocation during u8[]→string lowering")
	}
}
