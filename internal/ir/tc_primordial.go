package ir

import (
	"github.com/ameerthehacker/zeus/internal/token"
	"github.com/ameerthehacker/zeus/internal/zeus_value"
)

// PrimordialClassGenPass collects all array types and user-defined types that match primordial class names
// and generates class declarations for them at the top level
type PrimordialClassGenPass struct {
	arrayTypes      map[string]zeus_value.ArrayType
	primordialTypes map[string]*token.Span // tracks user-defined types matching primordial names
}

func NewPrimordialClassGenPass() *PrimordialClassGenPass {
	return &PrimordialClassGenPass{
		arrayTypes:      make(map[string]zeus_value.ArrayType),
		primordialTypes: make(map[string]*token.Span),
	}
}

func (p *PrimordialClassGenPass) GetName() string {
	return "PrimordialClassGenPass"
}

func (p *PrimordialClassGenPass) HandleInstruction(tc *TypeChecker, instr *Instr) {
	// Collect types from instruction outputs
	if instr.Output != nil && instr.Output.ValueType != nil {
		p.collectType(instr.Output.ValueType)
	}

	// Collect from specific instruction types
	switch instr.Type {
	case InstrTypeDeclVar, InstrTypeDeclGlobalVar:
		input := AsDeclVarInstrInput(instr.Input)
		p.collectType(input.Variable.ValueType)
	case InstrTypeDeclFunc:
		input := AsDeclFuncInstrInput(instr.Input)
		p.collectType(input.Function.ReturnType)
		for _, param := range input.Function.Params {
			p.collectType(param.ValueType)
		}
	case InstrTypeDeclPrimordialFunc:
		input := AsDeclPrimordialFuncInstrInput(instr.Input)
		p.collectType(input.Function.ReturnType)
		for _, param := range input.Function.Params {
			p.collectType(param.ValueType)
		}
	case InstrTypeDeclClass:
		input := AsDeclClassInstrInput(instr.Input)
		for _, property := range input.Class.Properties {
			p.collectType(property.Property.ValueType)
		}
		if input.Class.ArrayElementType != nil {
			p.collectType(input.Class.ArrayElementType)
		}
	case InstrTypeDeclClassMethod:
		input := AsDeclClassMethodInstrInput(instr.Input)
		p.collectType(input.Method.ReturnType)
		for _, param := range input.Method.Params {
			p.collectType(param.ValueType)
		}
	}
}

func (p *PrimordialClassGenPass) collectType(valueType zeus_value.ValueType) {
	if zeus_value.IsArrayType(valueType) {
		p.collectArrayType(valueType)
	} else if zeus_value.IsUserDefinedType(valueType) {
		p.collectUserDefinedType(valueType)
	}
}

func (p *PrimordialClassGenPass) collectUserDefinedType(valueType zeus_value.ValueType) {
	userDefinedType := zeus_value.AsUserDefinedType(valueType)
	// Check if this is a known primordial class name
	if p.isPrimordialClassName(userDefinedType.Name) {
		if _, exists := p.primordialTypes[userDefinedType.Name]; !exists {
			p.primordialTypes[userDefinedType.Name] = userDefinedType.Span
		}
	}
}

func (p *PrimordialClassGenPass) isPrimordialClassName(name string) bool {
	return name == zeus_value.ZEUS_PRIMORDIAL_STRING
}

func (p *PrimordialClassGenPass) collectArrayType(valueType zeus_value.ValueType) {
	if zeus_value.IsArrayType(valueType) {
		arrayType := zeus_value.AsArrayType(valueType)
		typeName := arrayType.String()

		// Only add if not already collected
		if _, exists := p.arrayTypes[typeName]; !exists {
			p.arrayTypes[typeName] = *arrayType
		}

		// Recursively collect element types if they are also arrays or user-defined types
		p.collectType(arrayType.ElementType)
	}
}

// emittedClasses tracks which primordial classes have been emitted to avoid duplicates
var emittedClasses = make(map[string]bool)

// primordialInsertionIndex tracks where to insert the next primordial class declaration
// This ensures dependencies are emitted in the correct order (dependencies first)
var primordialInsertionIndex = 0

func (p *PrimordialClassGenPass) Finalize(tc *TypeChecker) {
	// Reset trackers
	emittedClasses = make(map[string]bool)
	primordialInsertionIndex = 0

	// Pre-populate emittedClasses with classes already declared by IRBuilder.initializePrimordials
	// so we don't emit duplicate DECL_CLASS instructions for the same types
	for _, instr := range tc.builder.instrs {
		if instr.Type == InstrTypeDeclClass {
			class := AsDeclClassInstrInput(instr.Input).Class
			emittedClasses[class.Name] = true
		}
	}

	// Generate class declarations for all collected array types first
	// We need to sort them by depth (simpler types first) to ensure dependencies are resolved
	sortedTypes := p.sortArrayTypesByDepth()

	for _, arrayType := range sortedTypes {
		p.emitArrayClass(tc, arrayType)
	}

	// Emit DECL_CLASS instructions for primordial types that are used (like "string")
	for typeName, span := range p.primordialTypes {
		p.emitPrimordialClass(tc, typeName, span)
	}
}

// emitArrayClass emits a DECL_CLASS instruction for an array type if not already emitted
func (p *PrimordialClassGenPass) emitArrayClass(tc *TypeChecker, arrayType zeus_value.ArrayType) {
	typeName := arrayType.String()

	// Skip if already emitted
	if emittedClasses[typeName] {
		return
	}

	// Check if class already exists in symbol table (might be pre-registered)
	var arrayClass *zeus_value.Class
	if symbol, ok := tc.builder.symbolTable.GetSymbol(typeName); ok {
		arrayClass = zeus_value.AsClass(symbol)
	} else {
		// Create and register the array primordial class
		arrayClass = zeus_value.GetArrayPrimordialClassDefinition(arrayType)
		tc.builder.symbolTable.DeclareSymbol(typeName, arrayClass)
	}

	if arrayClass == nil {
		return
	}

	// Mark as emitted before emitting to handle circular dependencies
	emittedClasses[typeName] = true

	// Emit DECL_CLASS instruction at the current insertion index
	// This ensures proper ordering: dependencies come before dependents
	result := tc.builder.createTempVariable(arrayType.GetSpan())
	tc.builder.insertionIndex = primordialInsertionIndex
	tc.builder.currentBlock = nil
	tc.builder.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(arrayClass),
		Span:   arrayType.GetSpan(),
	})
	primordialInsertionIndex++
}

// emitPrimordialClass emits a DECL_CLASS instruction for a primordial class and its dependencies
func (p *PrimordialClassGenPass) emitPrimordialClass(tc *TypeChecker, typeName string, span *token.Span) {
	// Skip if already emitted
	if emittedClasses[typeName] {
		return
	}

	// Get the primordial class from symbol table (already registered in ir.go)
	symbol, ok := tc.builder.symbolTable.GetSymbol(typeName)
	if !ok {
		return
	}

	primordialClass := zeus_value.AsClass(symbol)
	if primordialClass == nil {
		return
	}

	// Handle transitive dependencies: emit dependent primordial classes first
	for _, property := range primordialClass.Properties {
		if zeus_value.IsObjectType(property.Property.ValueType) {
			objType := zeus_value.AsObjectType(property.Property.ValueType)
			if objType.Class.PrimordialName == zeus_value.ZEUS_PRIMORDIAL_ARRAY {
				// This is an array dependency, emit it first
				// Reconstruct the array type from the class
				if objType.Class.ArrayElementType != nil {
					depArrayType := zeus_value.ArrayType{
						ElementType: objType.Class.ArrayElementType,
						Span:        span,
					}
					p.emitArrayClass(tc, depArrayType)
				}
			} else if objType.Class.PrimordialName != "" {
				// This is another primordial class dependency
				p.emitPrimordialClass(tc, objType.Class.Name, span)
			}
		}
	}

	// Mark as emitted
	emittedClasses[typeName] = true

	// Emit DECL_CLASS instruction at the current insertion index
	result := tc.builder.createTempVariable(span)
	tc.builder.insertionIndex = primordialInsertionIndex
	tc.builder.currentBlock = nil
	tc.builder.pushInstr(&Instr{
		Type:   InstrTypeDeclClass,
		Output: result,
		Input:  NewDeclClassInstrInput(primordialClass),
		Span:   span,
	})
	primordialInsertionIndex++
}

// sortArrayTypesByDepth sorts array types so that simpler types (fewer dimensions) come first
func (p *PrimordialClassGenPass) sortArrayTypesByDepth() []zeus_value.ArrayType {
	var result []zeus_value.ArrayType

	// Calculate depth for each type
	type typeWithDepth struct {
		arrayType zeus_value.ArrayType
		depth     int
	}

	var typesWithDepth []typeWithDepth
	for _, arrayType := range p.arrayTypes {
		depth := p.calculateDepth(arrayType)
		typesWithDepth = append(typesWithDepth, typeWithDepth{arrayType, depth})
	}

	// Sort by depth (ascending)
	for i := 0; i < len(typesWithDepth); i++ {
		for j := i + 1; j < len(typesWithDepth); j++ {
			if typesWithDepth[i].depth > typesWithDepth[j].depth {
				typesWithDepth[i], typesWithDepth[j] = typesWithDepth[j], typesWithDepth[i]
			}
		}
	}

	for _, td := range typesWithDepth {
		result = append(result, td.arrayType)
	}

	return result
}

func (p *PrimordialClassGenPass) calculateDepth(arrayType zeus_value.ArrayType) int {
	depth := 1
	currentType := arrayType.ElementType

	for {
		if at, ok := currentType.(zeus_value.ArrayType); ok {
			depth++
			currentType = at.ElementType
		} else {
			break
		}
	}

	return depth
}
