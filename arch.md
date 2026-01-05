# Zeus Compiler Architecture

## Overview

Zeus is a statically-typed, compiled programming language with automatic garbage collection. The compiler is written in Go and uses LLVM for code generation. The runtime is implemented in Zig and provides garbage collection with precise stack scanning via LLVM statepoints.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       Zeus Compiler                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Source Code (.zs)                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │    Lexer     │  → Tokens                                    │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │    Parser    │  → AST (Abstract Syntax Tree)               │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │   Zeus IR    │  → Intermediate Representation              │
│  │  Generator   │                                              │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │ Type Checker │  → Type-checked IR                          │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │   LLVM IR    │  → LLVM Intermediate Representation         │
│  │  Generator   │                                              │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │ Optimization │  → PlaceSafepoints,                         │
│  │   Passes     │    RewriteStatepointsForGC                  │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │   Object     │  → .o files                                 │
│  │  Generator   │                                              │
│  └──────────────┘                                              │
│         ↓                                                        │
│  ┌──────────────┐                                              │
│  │    Linker    │  → Executable                               │
│  └──────────────┘                                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Compiler Phases

### 1. Lexical Analysis (Lexer)

**Location**: `internal/lexer/lexer.go`

**Purpose**: Converts raw source code into a stream of tokens.

**Key Features**:
- Handles identifiers, keywords, operators, literals (numbers, strings, booleans)
- Supports numeric separators (e.g., `2_00_00_000`)
- Multiple number bases (binary, octal, decimal, hexadecimal)
- Single-line comments (`//`)
- Position tracking for error reporting

**Token Types** (`internal/token/token.go`):
- Delimiters: `(`, `)`, `{`, `}`, `[`, `]`, `;`, `:`, `,`, `.`
- Operators: `+`, `-`, `*`, `/`, `=`, `!`, `==`, `!=`, `>`, `>=`, `<`, `<=`
- Keywords: `let`, `const`, `function`, `return`, `if`, `else`, `while`, `class`, `new`, `import`, `export`, etc.
- Data types: `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, `f64`, `boolean`, `void`, `null`
- Literals: numbers, strings, identifiers

### 2. Syntax Analysis (Parser)

**Location**: `internal/parser/parser.go`

**Purpose**: Converts token stream into an Abstract Syntax Tree (AST).

**Parser Design**:
- **Pratt Parser**: Uses precedence climbing for expression parsing
- **Recursive Descent**: For statement parsing
- **Error Recovery**: Synchronization mechanism to continue parsing after errors

**Parsing Strategy**:
- **Prefix Parselets**: Handle tokens at the start of expressions (literals, unary operators, parentheses)
- **Infix Parselets**: Handle binary operators, function calls, member access
- **Operator Precedence**:
  ```
  6: Member access (.)
  5: Function call, new
  4: Unary (-, !), Multiplication (*), Division (/)
  3: Addition (+), Subtraction (-), Comparison (<, >, <=, >=)
  2: Equality (==, !=)
  1: Assignment (=)
  ```

**AST Structure** (`internal/ast/`):
- **Expressions** (`expr.go`):
  - Literals: `NumberExprNode`, `BooleanExprNode`, `NullExprNode`
  - Binary: `BinaryExprNode`
  - Unary: `UnaryExprNode`
  - Identifiers: `IdentifierExprNode`
  - Functions: `FunctionDeclExprNode`, `FunctionCallExprNode`
  - Classes: `ClassDeclExprNode`, `NewExprNode`, `ObjectPropertyAccessExprNode`
  - Type expressions: `TypeExpressionNode` (for arrays)
  
- **Statements** (`stmt.go`):
  - Variable declarations: `VarDeclStmtNode`
  - Control flow: `IfStmtNode`, `WhileStmtNode`, `ReturnStmtNode`
  - Blocks: `BlockStmtNode`
  - Module system: `ImportStmtNode`, `ExportStmtNode`
  - Expression statements: `ExprStmtNode`

### 3. Zeus IR Generation

**Location**: `internal/ir/ir.go`, `internal/ir/builder.go`, `internal/ir/instr.go`

**Purpose**: Converts AST into a custom intermediate representation optimized for Zeus semantics.

**IR Design**:
- **Three-Address Code**: Most instructions have at most three operands
- **SSA-like**: Temporary variables are immutable (assigned once)
- **Basic Blocks**: Code organized into basic blocks with control flow edges
- **Symbol Tables**: Scoped symbol management for variables and functions

**Instruction Types** (`InstrType`):
- **Arithmetic**: `ADD`, `SUB`, `MUL`, `DIV`, `NEG`
- **Comparison**: `EQ_EQ`, `NOT_EQ`, `LESS_THAN`, `LESS_THAN_EQ`, `GREATER_THAN`, `GREATER_THAN_EQ`
- **Logical**: `NOT`
- **Memory**: `LOAD`, `STORE`, `DECLARE_VAR`
- **Control Flow**: `JMP`, `COND_JMP`, `RETURN`
- **Functions**: `DECLARE_FUNC`, `CALL_FUNC`, `CALL_INDIRECT_FUNC`
- **Classes**: `DECLARE_CLASS`, `DECLARE_CLASS_METHOD`, `NEW_OBJ`, `OBJECT_PROPERTY_ACCESS`
- **Modules**: `IMPORT`, `EXPORT`
- **Type Conversion**: `CAST`

**IR Generation Process**:
1. **Visitor Pattern**: Implements `ast.ExprVisitor` and `ast.StmtVisitor`
2. **Symbol Table Management**: Tracks variables, functions, and classes
3. **Module Resolution**: Handles imports and exports
4. **Class Processing**: Generates class structures, methods, and constructors

**Key Features**:
- **Primordial Classes**: Built-in classes (Array) with runtime implementations
- **Method Name Mangling**: `ClassName.methodName` for disambiguation
- **Scope Management**: Nested scopes for blocks, functions, and classes
- **Circular Dependency Detection**: Prevents infinite import loops

### 4. Type Checking

**Location**: `internal/ir/tc.go`

**Purpose**: Validates types and performs semantic analysis.

**Architecture**:
- **Pass-Based System**: Pluggable type checking passes
- **Context Management**: Tracks current function, class, and block

**Type Checking Passes**:

1. **ToKnownTypesPass**:
   - Resolves user-defined types to their actual types
   - Converts array type syntax to object types
   - Validates type usage (e.g., void only as return type)
   - Resolves class and function types

2. **TypeCheckingPass**:
   - **Type Compatibility**: Checks operand types for operations
   - **Implicit Casting**: 
     - Integer to float
     - Smaller int to larger int (with sign considerations)
     - Smaller float to larger float
     - Null to object types
   - **Function Call Validation**: Parameter count and types
   - **Return Value Checking**: Ensures all code paths return
   - **Class Validation**: Constructor signatures, access modifiers
   - **Entry Point**: Validates `main` function exists in entry modules

3. **UnusedWarningPass**:
   - Tracks variable, function, and class usage
   - Generates warnings for unused declarations
   - Excludes temporary variables and system functions

**Type System**:
- **Primitive Types**: `i8`, `i16`, `i32`, `i64`, `u8`, `u16`, `u32`, `u64`, `f32`, `f64`, `boolean`
- **Reference Types**: Objects, arrays
- **Special Types**: `void` (return only), `null` (assignable to objects)
- **Function Types**: First-class function support
- **User-Defined Types**: Classes

### 5. LLVM IR Generation

**Location**: `internal/codegen/codegen.go`

**Purpose**: Translates Zeus IR to LLVM IR for optimization and code generation.

**Code Generation Strategy**:

**Type Mapping**:
```go
// Primitives
i8  → LLVM i8
i16 → LLVM i16
i32 → LLVM i32
i64 → LLVM i64
f32 → LLVM float
f64 → LLVM double
boolean → LLVM i1

// Objects → Pointer to struct (address space 1 for GC)
// Functions → Pointer to function
```

**Class Representation**:

Each Zeus class generates three LLVM structs:

1. **VTable Struct**: Function pointers for virtual methods
   ```llvm
   %ClassName_vtable = type { ptr, ptr, ... }
   ```

2. **Object Header Struct**: Metadata for GC and runtime
   ```llvm
   %ClassName_header = type {
     ptr,                    ; vtable pointer
     ptr,                    ; type info
     i8,                     ; gc offsets count
     [n x i8]                ; gc offsets array
   }
   ```

3. **Class Struct**: Actual object layout
   ```llvm
   %ClassName = type {
     ptr,                    ; object header pointer
     <field1_type>,         ; property fields
     <field2_type>,
     ...
   }
   ```

**Memory Management**:
- **Allocation**: `zeus_gc_alloc(size)` returns GC-managed memory
- **Address Space 1**: All GC objects use address space 1
- **Object Headers**: Prepended to all objects for GC tracking

**GC Integration**:
```llvm
; All functions that allocate set GC strategy
define void @myFunc() gc "statepoint-example" {
  ; Function body
}

; GC safepoint poll inserted by LLVM
declare void @gc.safepoint_poll()
```

**Primordial Classes**:
- Built-in classes (like Array) have wrapper methods
- Wrappers call Zig runtime functions
- Marked with `alwaysinline` attribute for optimization
- Use ABI-compatible calling convention

**Module System**:
- **Scoped Names**: Exported symbols prefixed with module path
- **External Linkage**: Exported functions/classes visible across modules
- **Private Linkage**: Internal functions hidden from other modules

### 6. LLVM Optimization Passes

**Location**: `internal/zeus_compiler/compiler.go` (`RunOptimizationPasses`)

**Optimization Pipeline**:

1. **mem2reg**: 
   - Promotes allocas to SSA registers
   - Essential for performance

2. **place-safepoints** (if GC enabled):
   - Inserts GC safepoint polls at:
     - Function entries
     - Loop backedges
     - Before potentially allocating calls

3. **rewrite-statepoints-for-gc** (if GC enabled):
   - Transforms function calls to statepoint intrinsics
   - Preserves GC root information
   - Enables precise stack scanning

**Statepoint Transformation**:
```llvm
; Before
%result = call ptr @allocate()

; After
%statepoint_token = call token (i64, i32, ptr, i32, i32, ...)
  @llvm.experimental.gc.statepoint.p0(
    i64 0, i32 0, ptr @allocate,
    i32 0, i32 0, i32 0, i32 0
  )
%result = call ptr @llvm.experimental.gc.result.p0(
  token %statepoint_token
)
```

### 7. Object File Generation and Linking

**Object Generation** (`EmitObjFiles`):
1. Generates temporary object files for each module
2. Uses LLVM's target machine to emit machine code
3. Outputs LLVM IR files in debug mode

**Linking** (`LinkObjFiles`):
- **Linker**: Uses `clang` on macOS, `gcc` on Linux
- **Runtime**: Links with Zeus runtime (`zeus-runtime.o`)
- **System Libraries**: Automatically links required libraries
- **Platform-Specific**:
  - macOS: Sets deployment target (12.0), SDK path
  - Handles different Xcode and Command Line Tools installations

## Runtime System

**Location**: `runtime/` (Zig implementation)

### Garbage Collection

**Type**: Mark-and-sweep with precise stack scanning

**Components**:

1. **GC Allocator** (`gc.zig`):
   ```zig
   pub const GC = struct {
     allocator: std.mem.Allocator,
     gc_roots: ArrayList(*ZeusObj),
     allocated_objects: ArrayList(AllocatedObject),
     alloc_mutex: std.Thread.Mutex,
   }
   ```

2. **Stack Walking** (`stackmap.zig`):
   - Uses **libunwind** for robust stack traversal
   - Parses LLVM-generated stack maps
   - Extracts GC root pointers from stack frames

3. **GC Runtime** (`gc_runtime.zig`):
   - **`zeus_gc_alloc(size)`**: Allocates GC-tracked memory
   - **`zeus_gc_poll()`**: Triggers GC cycle
     1. Walk stack and collect roots
     2. Register roots with GC
     3. Run mark-and-sweep

**GC Algorithm**:
1. **Mark Phase**:
   - Start from GC roots (stack-scanned pointers)
   - Recursively mark reachable objects
   - Follow object header GC offsets for nested objects
   - Handle arrays with object elements specially

2. **Sweep Phase**:
   - Iterate through allocated objects
   - Free unmarked objects
   - Cleanup array data buffers
   - Update tracking structures

### Object Layout (Zeus ABI)

**Object Header** (`runtime/abi.zig`):
```zig
pub const ZeusObjectHeader = extern struct {
  vtable_ptr: *anyopaque,           // VTable pointer
  type_info: *ZeusObjectTypeInfo,   // Runtime type info
  gc_offsets_count: u8,             // Number of GC-tracked fields
  gc_offsets: [*]u8,                // Byte offsets to GC fields
};
```

**Object Structure**:
```zig
pub const ZeusObj = extern struct {
  obj_header: *ZeusObjectHeader,    // Header pointer
  // Object fields follow
};
```

**Type Information**:
```zig
pub const ZeusObjectTypeInfo = extern struct {
  type_id: u8,                      // Unique type ID
  object_type: ZeusObjectType,      // Object or Array
  array_element_type: ZeusType,     // Element type for arrays
  parent_type_info: ?*ZeusObjectTypeInfo,  // For inheritance
};
```

### Array Implementation

Arrays in Zeus are implemented as **primordial classes** - first-class objects with built-in methods and type-safe operations. This design provides dynamic sizing, multi-dimensional support, and seamless GC integration while maintaining performance.

#### Design Philosophy

Zeus arrays combine the ergonomics of high-level languages (JavaScript, Python) with the performance characteristics of systems languages (C++, Rust):

- **Dynamic**: Can grow/shrink at runtime
- **Type-safe**: Element type tracked at compile-time and runtime
- **Multi-dimensional**: `u8[][]`, `Point[][][]` fully supported
- **Dual syntax**: Bracket notation (`arr[i]`) and method calls (`arr.get(i)`)
- **GC-managed**: Both array object and data buffer are tracked

#### Type System Representation

**Location**: `internal/zeus_value/value_type.go`

Arrays are represented as recursive types:

```go
type ArrayType struct {
    ElementType ValueType  // Can be any type, including another ArrayType
    Span *token.Span
}

func (a ArrayType) String() string {
    return fmt.Sprintf("%s[]", a.ElementType.String())
}
```

**Multi-dimensional arrays** work via nesting:
- `u8[][]` = `ArrayType{ElementType: ArrayType{ElementType: u8}}`
- `Point[][][]` = `ArrayType{ElementType: ArrayType{ElementType: ArrayType{ElementType: Point}}}`

This compositional approach is clean and supports arbitrary nesting depth.

#### Primordial Class Definition

**Location**: `internal/zeus_value/primordials.go`

Each array type (e.g., `u8[]`, `Point[]`) gets its own class definition:

```go
func GetArrayPrimordialClassDefinition(arrayType ArrayType) *Class {
    // Three private properties
    capacityProperty := NewClassProperty(
        NewVar("capacity", IntType{Size: I32}, false),
        &token.Token{Type: token.TokenTypePrivate}
    )
    lengthProperty := NewClassProperty(
        NewVar("length", IntType{Size: I32}, false),
        &token.Token{Type: token.TokenTypePrivate}
    )
    dataProperty := NewClassProperty(
        NewVar("data", OpaqueType{}, true),  // GC-tracked pointer
        &token.Token{Type: token.TokenTypePrivate}
    )
    
    // Public methods
    constructorMethod := NewFunction("constructor", 
        []*Var{NewVar("capacity", IntType{Size: I32})},
        VoidType{}
    )
    pushMethod := NewFunction("push",
        []*Var{NewVar("value", arrayType.ElementType)},
        VoidType{}
    )
    popMethod := NewFunction("pop", []*Var{}, arrayType.ElementType)
    getMethod := NewFunction("get",
        []*Var{NewVar("index", IntType{Size: I32})},
        arrayType.ElementType
    )
    setMethod := NewFunction("set",
        []*Var{
            NewVar("index", IntType{Size: I32}),
            NewVar("value", arrayType.ElementType)
        },
        VoidType{}
    )
    
    // Return class with all methods
    return NewClass(arrayClassName, properties, methods, "array", arrayType)
}
```

**Key insight**: Each array type is a distinct class, allowing type-specific operations while sharing runtime implementation.

#### Parser Implementation

**Location**: `internal/parser/parser.go`

Arrays use **indexing expressions** for both type declarations and element access:

```go
// Type declaration: u8[][]
func (p *Parser) consumeDataType() *ast.ValueTypeNode {
    // Parse base type: u8
    valueType := zeus_value.ToValueType(nextToken)
    
    // Parse array dimensions: [][]
    for p.peek().Type == token.TokenTypeLeftBracket {
        p.consume()  // [
        p.consumeToken(token.TokenTypeRightBracket)  // ]
        valueType = zeus_value.ArrayType{ElementType: valueType}
    }
    
    return &ast.ValueTypeNode{ValueType: valueType}
}

// Array indexing: arr[0][1]
func (p *Parser) consumeIndexingMetadata() *ast.IndexingMeta {
    indexingExprs := []ast.ExprNode{}
    
    // First index ([ already consumed by infix parselet)
    indexExpr := p.parseExprOfPrecedence(0, true)  // Optional for type exprs
    indexingExprs = append(indexingExprs, indexExpr)
    p.consumeToken(token.TokenTypeRightBracket)
    
    // Chained indices: [1], [2], ...
    for p.peek().Type == token.TokenTypeLeftBracket {
        p.consumeToken(token.TokenTypeLeftBracket)
        indexExpr := p.parseExprOfPrecedence(0, true)
        indexingExprs = append(indexingExprs, indexExpr)
        p.consumeToken(token.TokenTypeRightBracket)
    }
    
    return &ast.IndexingMeta{IndexingExprs: indexingExprs}
}
```

The parser supports **chained indexing** seamlessly: `arr[i][j][k]` parses as a single `IndexingExprNode` with three index expressions.

#### Syntactic Desugaring (IR Generation)

**Location**: `internal/ir/ir.go`

The most sophisticated part is how **bracket notation desugars to method calls**:

```go
func (g *IRModule) VisitIndexingExpression(expr *ast.IndexingExprNode) zeus_value.Value {
    wasLValueExpr := g.isLValueExpr
    g.isLValueExpr = false
    currentValue := expr.Array.Accept(g)
    g.isLValueExpr = wasLValueExpr
    
    // Reading: arr[0][1] → temp = arr.get(0); result = temp.get(1)
    if !g.isLValueExpr {
        for _, indexExpr := range expr.IndexingMeta.IndexingExprs {
            index := indexExpr.Accept(g)
            
            // Cast index to i32 if needed
            if intType := zeus_value.AsIntType(zeus_value.GetValueType(index)); 
               intType != nil && intType.Size != zeus_value.I32 {
                index = g.irBuilder.BuildCast(index, 
                    zeus_value.IntType{Size: zeus_value.I32})
            }
            
            // Get .get() method and call it
            getMethodPtr := g.irBuilder.BuildObjectPropertyAccess(
                currentValue, "get", false)
            getMethod := g.irBuilder.BuildLoad(getMethodPtr)
            currentValue = g.irBuilder.BuildIndirectFuncCall(
                getMethod, []zeus_value.Value{index})
        }
        return currentValue
    }
    
    // Writing: arr[0][1] = value
    // Process all but last index with .get()
    for i := 0; i < len(expr.IndexingMeta.IndexingExprs)-1; i++ {
        index := expr.IndexingMeta.IndexingExprs[i].Accept(g)
        // ... cast to i32 ...
        getMethodPtr := g.irBuilder.BuildObjectPropertyAccess(
            currentValue, "get", false)
        getMethod := g.irBuilder.BuildLoad(getMethodPtr)
        currentValue = g.irBuilder.BuildIndirectFuncCall(
            getMethod, []zeus_value.Value{index})
    }
    
    // Return ArrayElementRef for last index
    // Assignment handler will generate .set() call
    lastIndex := expr.IndexingMeta.IndexingExprs[last].Accept(g)
    return zeus_value.NewArrayElementRef(currentValue, lastIndex)
}
```

**Assignment handling**:

```go
func (g *IRModule) VisitBinaryExpr(expr *ast.BinaryExprNode) zeus_value.Value {
    if expr.Operator.Type == token.TokenTypeEqual {
        if arrayRef := zeus_value.AsArrayElementRef(left); arrayRef != nil {
            // arr[i] = value → arr.set(i, value)
            setMethodPtr := g.irBuilder.BuildObjectPropertyAccess(
                arrayRef.ArrayObject, "set", false)
            setMethod := g.irBuilder.BuildLoad(setMethodPtr)
            g.irBuilder.BuildIndirectFuncCall(
                setMethod, []zeus_value.Value{arrayRef.Index, right})
            return right
        }
    }
}
```

**Example transformation**:

```zeus
// Source
let arr: u8[][] = new u8[][];
arr[0] = new u8[10];
arr[0][5] = 42;
let x = arr[0][5];

// Desugared IR (conceptual)
let arr = new u8[][];
let temp1 = arr.get(0);
temp1.set(5, 42);
let temp2 = arr.get(0);
let x = temp2.get(5);
```

This approach elegantly handles:
- **Read vs write contexts** via `isLValueExpr` flag
- **Multi-dimensional arrays** via recursive `.get()` calls
- **Chained assignments** via `ArrayElementRef` intermediate value

#### LLVM Code Generation

**Location**: `internal/codegen/codegen.go`

**Memory Layout**:

Arrays compile to LLVM structs matching the Zig ABI:

```llvm
; Array object struct
%u8[] = type {
    ptr addrspace(1),  ; obj_header pointer (GC managed)
    i32,               ; capacity
    i32,               ; length
    ptr                ; data buffer pointer
}
```

**Optimization: Unified Object Array Class**

All object arrays (`Point[]`, `MyClass[][]`) share a single LLVM struct definition:

```go
func (c *CodegenModule) genClass(class zeus_value.Class) *ZeusClassLLVMStruct {
    if class.PrimordialName == "array" && class.Name != "object[]" {
        // Reuse object[] struct for all object array types
        // Point*, MyClass* have same size/representation
        c.zeusClassLLVMStructMap[class.Name] = c.genObjArrayClass()
        return c.zeusClassLLVMStructMap[class.Name]
    }
    // ... generate unique structs for primitive arrays ...
}
```

This reduces code bloat while maintaining type safety at the IR level.

**Primordial Method Wrappers**:

Array methods are thin wrappers around Zig runtime functions:

```go
func (c *CodegenModule) genPrimordialClassMethods(class zeus_value.Class) {
    for _, method := range class.Methods {
        classFunction := c.genClassMethod(method, class)
        
        // Mark as alwaysinline for zero overhead
        alwaysInlineAttr := c.cxt.CreateEnumAttribute(
            llvm.AttributeKindID("alwaysinline"), 0)
        classFunction.AddAttributeAtIndex(-1, alwaysInlineAttr)
        
        // Generate wrapper body:
        // 1. Get runtime function pointer
        runtimeFunc := c.genPrimordialRuntimeFunction(
            method, "zeus_array_" + method.Name)
        
        // 2. Prepare arguments (this, return_buffer, params)
        args := []llvm.Value{thisPtr, returnBufferPtr}
        for _, param := range method.Params {
            paramAlloca := c.builder.CreateAlloca(...)
            c.builder.CreateStore(param, paramAlloca)
            args = append(args, paramAlloca)
        }
        
        // 3. Call runtime function
        c.builder.CreateCall(runtimeFunc, args)
        
        // 4. Extract and return result
        if !isVoid(method.ReturnType) {
            returnPtr := c.builder.CreateLoad(returnBufferPtr)
            result := c.extractResultFromBuffer(returnPtr, method.ReturnType)
            c.builder.CreateRet(result)
        }
    }
}
```

The `alwaysinline` attribute ensures these wrappers are completely eliminated, resulting in direct runtime calls with **zero overhead**.

#### Runtime Implementation

**Location**: `runtime/array_runtime.zig`

**Array Object ABI**:

```zig
pub const ZeusArrayObj = extern struct {
  obj_header: *ZeusObjectHeader,
  capacity: u32,         // Allocated capacity
    length: u32,           // Current element count
  data: ?*anyopaque,     // Pointer to data buffer
};
```

**Growth Strategy**:

Uses standard **doubling strategy** with minimum capacity:

```zig
const ARRAY_GROWTH_FACTOR: u32 = 2;
const ARRAY_MIN_CAPACITY: u32 = 4;

export fn zeus_array_push(this_ptr: *anyopaque, ...) void {
    const array_ptr = castToArrayObj(this_ptr);
    
    if (array_ptr.length >= array_ptr.capacity) {
        const new_capacity = if (array_ptr.capacity == 0)
            ARRAY_MIN_CAPACITY
        else
            array_ptr.capacity * ARRAY_GROWTH_FACTOR;
        
        if (!resizeArray(array_ptr, new_capacity)) return;
    }
    
    // Copy element to data[length]
    const offset = array_ptr.length * element_size;
    @memcpy(data_bytes[offset..], value_bytes[0..element_size]);
    array_ptr.length += 1;
}
```

Same as C++ `std::vector`, Rust `Vec`, Java `ArrayList`.

**Bounds Checking**:

```zig
export fn zeus_array_get(this_ptr: *anyopaque, 
                          return_buffer_ptr: ?*anyopaque,
                          index_ptr: *anyopaque) void {
    const array_ptr = castToArrayObj(this_ptr);
    const index = @as(*i32, @ptrCast(index_ptr)).*;
    
    // Bounds check
    if (index < 0 or index >= array_ptr.length) {
        // Return zeroed buffer (safe default)
        _ = allocateZeroedReturnBuffer(return_buffer_ptr, element_size);
        debug.log("array_get: index {} out of bounds for length {}", 
                  .{index, array_ptr.length});
        return;
    }
    
    // Copy element to return buffer
    const offset = @as(u32, @intCast(index)) * element_size;
    const result_bytes = allocateReturnBuffer(return_buffer_ptr, element_size);
    @memcpy(result_bytes, data_bytes[offset..offset + element_size]);
}
```

**Auto-growing on set**:

```zig
export fn zeus_array_set(this_ptr: *anyopaque, ...) void {
    const array_ptr = castToArrayObj(this_ptr);
    const index = @as(*i32, @ptrCast(index_ptr)).*;
    
    if (index < 0) return;  // Reject negative indices
    
    const target_index = @as(u32, @intCast(index));
    
    // Grow if necessary to fit index
    if (target_index >= array_ptr.capacity) {
        var new_capacity = if (array_ptr.capacity == 0) 
            ARRAY_MIN_CAPACITY 
        else 
            array_ptr.capacity;
            
        while (new_capacity <= target_index) {
            new_capacity *= ARRAY_GROWTH_FACTOR;
        }
        
        if (!resizeArray(array_ptr, new_capacity)) return;
    }
    
    // Set element
    @memcpy(data_bytes[offset..], value_bytes[0..element_size]);
    
    // Update length if we set beyond current length
    if (target_index >= array_ptr.length) {
        array_ptr.length = target_index + 1;
    }
}
```

This allows sparse array initialization: `arr[100] = 42` automatically allocates space.

**Default Value Initialization**:

When arrays grow, new slots are initialized with type-appropriate defaults:

```zig
fn writeDefaultValue(dest: [*]u8, zeus_type: ZeusType, size: u32) void {
    switch (zeus_type) {
        ._i8, ._i16, ._i32, ._i64 => { 
            // Zero for integers
            @memset(dest[0..size], 0);
        },
        ._f32, ._f64 => {
            // 0.0 for floats
            const ptr = @as(*f64, @ptrCast(dest));
            ptr.* = 0.0;
        },
        ._bool => {
            const ptr = @as(*bool, @ptrCast(dest));
            ptr.* = false;
        },
        .object => {
            // null for objects
            const ptr = @as(*?*anyopaque, @ptrCast(dest));
            ptr.* = null;
        },
    }
}
```

#### Garbage Collection Integration

**Dual Cleanup Responsibility**:

Arrays have two separate memory regions:
1. **Array object struct** - allocated via GC allocator
2. **Data buffer** - allocated via array runtime allocator

**GC Sweep Phase** (`runtime/gc.zig`):

```zig
fn sweep(self: *GC) void {
    for (self.allocated_objects.items) |*obj| {
        if (!obj.marked) {
            // Call cleanup for arrays before freeing object
            if (obj.ptr.obj_header.getObjectTypeInfo().object_type == .array) {
                array_runtime.zeus_array_cleanup(obj.ptr);
            }
            
            // Free object struct
            const bytes = @as([*]u8, @ptrCast(obj.ptr))[0..obj.size];
            self.allocator.free(bytes);
        }
    }
}
```

**Array Cleanup** (`runtime/array_runtime.zig`):

```zig
pub fn zeus_array_cleanup(array_obj_ptr: *anyopaque) callconv(.C) void {
    const array_ptr = castToArrayObj(array_obj_ptr);
    
    if (array_ptr.data != null and array_ptr.capacity > 0) {
        const element_size = getElementSize(array_ptr);
        const data_size = array_ptr.capacity * element_size;
        const data_bytes = @as([*]u8, @ptrCast(array_ptr.data.?))[0..data_size];
        
        // Free data buffer
        allocator.free(data_bytes);
        
        array_ptr.data = null;
        array_ptr.length = 0;
        array_ptr.capacity = 0;
    }
}
```

**Object Array Traversal**:

For arrays containing objects (`Point[]`, `MyClass[]`), the GC must trace into the data buffer:

```zig
fn markObject(self: *GC, ptr: *ZeusObj) void {
    // ... mark object ...
    
    const type_info = obj.ptr.obj_header.getObjectTypeInfo();
    
    // Special handling for object arrays
    if (type_info.object_type == .array and 
        type_info.array_element_type == .object) {
        const array_obj = @as(*ZeusArrayObj, @ptrCast(obj.ptr));
        
        if (array_obj.data) |data| {
            // Interpret data buffer as array of object pointers
            const elements = @as([*]?*ZeusObj, @ptrCast(data));
            
            // Mark each non-null element
            for (elements[0..array_obj.length], 0..) |element, index| {
                if (element) |elem| {
                    self.markObject(elem);
                }
            }
        }
    }
}
```

This ensures objects stored in arrays are kept alive by the GC.

#### User-Facing Syntax

**Dual Syntax Support**:

```zeus
// Method syntax (explicit)
let arr: u8[] = new u8[10];
arr.push(1);
arr.push(2);
let x = arr.get(0);     // x = 1
arr.set(1, 42);         // arr[1] = 42

// Bracket syntax (syntactic sugar)
let arr2: u8[] = new u8[10];
arr2[0] = 10;
arr2[1] = 20;
let y = arr2[0];        // y = 10

// Multi-dimensional
let matrix: i32[][] = new i32[][];
matrix[0] = new i32[5];
matrix[0][0] = 100;
let z = matrix[0][0];   // z = 100
```

Both styles compile to identical IR - the choice is purely stylistic.

#### Comparison with Other Languages

**vs JavaScript/Python**:
- ✅ Similar ergonomics (methods, dynamic sizing)
- ✅ Static typing in Zeus
- ✅ Better performance (compiled, no interpreter overhead)

**vs Java**:
- ✅ Zeus arrays are dynamic (Java arrays are fixed-size)
- ✅ Dual syntax (Java only has `arr[i]`)
- ✅ Explicit capacity control

**vs C++ std::vector**:
- ✅ Similar performance (same growth strategy)
- ✅ GC-managed (no manual memory management)
- ✅ Built-in bounds checking
- ✅ Cleaner syntax (no template angle brackets)

**vs Rust Vec<T>**:
- ✅ Similar type safety and performance
- ❌ Zeus uses GC instead of borrow checker
- ✅ Simpler mental model (no lifetimes)
- ✅ Auto-initialized elements

**vs Swift Array**:
- ✅ Very similar design philosophy!
- ✅ Both have dynamic arrays as first-class types
- ✅ Both provide dual syntax
- ❌ Swift uses copy-on-write (Zeus doesn't yet)
- ✅ Zeus exposes capacity explicitly

**Overall**: Zeus arrays align most closely with **Swift** and modern managed languages while maintaining performance characteristics of **C++ std::vector**.

#### Performance Characteristics

**Time Complexity**:
- `push()`: Amortized O(1) (occasional O(n) reallocation)
- `pop()`: O(1)
- `get(i)`: O(1)
- `set(i, value)`: O(1) average, O(n) worst case (auto-grow)
- Multi-dimensional access `arr[i][j]`: O(d) for depth d

**Space Complexity**:
- O(capacity) for data buffer
- Unused capacity typically ≤ 50% (after doubling)

**Optimization Opportunities**:
1. ✅ **Inlined wrappers** - zero function call overhead
2. ✅ **Unified object arrays** - reduced code size
3. ⚠️ **Bounds check elimination** - possible in release mode
4. ⚠️ **Iterator support** - enable better optimizations
5. ⚠️ **Copy-on-write** - reduce copies for large arrays

#### Future Enhancements

**Planned Features**:
1. **Array literals**: `let arr = [1, 2, 3, 4, 5];`
2. **Slicing**: `arr[start:end]` for sub-arrays
3. **Iterator protocol**: Enable functional operations
4. **Convenience methods**: `.resize()`, `.reserve()`, `.clear()`
5. **Copy-on-write**: For efficient pass-by-value
6. **SIMD optimizations**: For primitive array operations
7. **Release mode optimizations**: Remove bounds checks

#### Key Files

**Compiler**:
- `internal/zeus_value/value_type.go` - Array type definition
- `internal/zeus_value/primordials.go` - Primordial class generation
- `internal/parser/parser.go` - Array syntax parsing
- `internal/ir/ir.go` - Bracket notation desugaring
- `internal/codegen/codegen.go` - LLVM struct generation

**Runtime**:
- `runtime/abi.zig` - Array ABI definition
- `runtime/array_runtime.zig` - Array operations implementation
- `runtime/gc.zig` - Array-aware garbage collection

**Tests**:
- `test/e2e/specs/array/` - E2E array tests
- `test/e2e/specs/gc/` - GC integration tests

### Runtime ABI Conventions

**Primordial Method Calling Convention**:
```c
void method(
  void* this_ptr,           // Object pointer
  void* return_buffer_ptr,  // Pointer to return value location
  void* param1_ptr,         // Pointer to parameter 1
  void* param2_ptr,         // Pointer to parameter 2
  ...
);
```

**Return Value Handling**:
- Caller allocates return buffer
- Runtime writes result to buffer
- Wrapper function extracts and returns value

## Module System

**Import/Export Mechanism**:

1. **Module Resolution** (`internal/module/module.go`):
   - Resolves relative paths
   - Handles standard library (`@std/...`)
   - Circular dependency detection

2. **Dependency Graph**:
   - BFS traversal of import tree
   - Topological ordering for compilation
   - Each source file maintains its own IR and symbols

3. **Symbol Scoping**:
   - Exported symbols have module-scoped names
   - Private symbols use internal linkage
   - Import/export tracked in IR

**Compilation Order**:
```
Entry File (main.zs)
  ↓ imports
Module A
  ↓ imports
Module B
```

Compiled in reverse order: B → A → main

## Type System Details

### Primitive Types

**Integer Types**:
- Signed: `i8`, `i16`, `i32`, `i64`
- Unsigned: `u8`, `u16`, `u32`, `u64`
- Size inference from literal values

**Floating Point**:
- `f32` (single precision)
- `f64` (double precision)

**Other**:
- `boolean` (true/false)
- `void` (function return only)
- `null` (object default value)

### Complex Types

**Arrays**:
```zeus
let arr: i32[] = new i32[10];  // Fixed-size array
arr.push(42);                   // Dynamic operations
```

**Classes**:
```zeus
class Point {
  public x: f32;
  public y: f32;
  
  public constructor(x: f32, y: f32): void {
    this.x = x;
    this.y = y;
  }
  
  public distance(): f32 {
    return (this.x * this.x + this.y * this.y);
  }
}

let p: Point = new Point(3.0, 4.0);
```

**Functions as Values**:
```zeus
function add(a: i32, b: i32): i32 {
  return a + b;
}

let fn: function(i32, i32): i32 = add;
```

### Type Inference and Casting

**Implicit Casts**:
- Integer to float
- Smaller to larger integers (when safe)
- Null to any object type
- Array type to object type (for built-in arrays)

**Explicit Casts**:
- Generated during type checking
- Inserted as `CAST` instructions in IR
- Compiled to appropriate LLVM cast instructions

## Error Handling

### Compile-Time Errors

**Error Reporting** (`internal/zeus_error/`, `internal/logger/`):
- **Severity Levels**: Error, Warning
- **Error Tracking**: Per-source-file error lists
- **Pretty Printing**: Shows source context with error location

**Error Types**:
- **Lexer Errors**: Unknown tokens, unterminated strings
- **Parser Errors**: Syntax errors, unexpected tokens
- **Semantic Errors**: Type mismatches, undefined symbols
- **Module Errors**: Circular dependencies, missing imports

**Example Error Output**:
```
error: type 'f32' is not assignable to type 'i32'
  --> main.zs:5:10
   |
 5 |   let x: i32 = 3.14;
   |          ^^^
```

### Runtime Errors

Currently minimal runtime error handling:
- Array bounds checking (returns default value)
- Null pointer safety (planned)

## Build System

**Makefile** (`makefile`):
- **Commands**:
  - `make build-runtime`: Builds Zig runtime
  - `make compile file=<name>`: Compiles Zeus file
  - `make run file=<name>`: Compiles and runs
  - `make test`: Runs all tests
  - `make test-e2e`: E2E tests

**Runtime Build** (`runtime/build.zig`):
- Zig build system
- Links with libunwind and libc
- Produces static library and object file
- Support for debug and release modes

**Compiler Entry** (`zeus.go`, `cmd/`):
- **CLI**: Built with Cobra
- **Commands**: `zeus build [file]`
- **Flags**: `-o` (output), `--target-dir` (temp directory)

## Testing Infrastructure

### E2E Tests

**Location**: `test/e2e/`

**Test Specs** (`specs/`):
- JSON spec files define expected output
- Test Zeus programs organized by feature
- Automatic compilation and execution
- Output comparison

**Categories**:
- Arrays (basic, 2D, 3D, object arrays)
- Classes (basic, methods, constructors)
- Functions (calls, void returns, exit)
- GC (basic, nested objects)
- Control flow (if/else, while loops)
- Variables (declarations, default values)

### Unit Tests

- **Lexer Tests**: Token generation
- **Parser Tests**: AST structure
- **Symbol Table Tests**: Scope management
- **Runtime Tests**: GC, array operations

## Development Workflow

**Debug Mode** (`ZEUS_DEBUG=true`):
- Prints Zeus IR for each module
- Outputs LLVM IR files
- Verbose GC logging (`ZEUS_GC_DEBUG=true`)

**Release Mode**:
- Optimized runtime (`make build-runtime release=true`)
- Stripped binaries
- No debug output

**Disable GC** (`ZEUS_NO_GC=true`):
- Skips GC-related LLVM passes
- Useful for testing without GC overhead

## Platform Support

**Current**:
- macOS (primary development platform)
- Linux (supported)

**Requirements**:
- LLVM 13+
- Zig 0.11+
- libunwind
- Clang/GCC

**Architecture**:
- x86_64
- ARM64 (Apple Silicon)

## Performance Considerations

**Optimization Strategies**:
1. **LLVM Optimizations**: Standard LLVM optimization passes
2. **Inlining**: Primordial methods marked `alwaysinline`
3. **Memory Layout**: Cache-friendly object layout
4. **GC Tuning**: Generational GC (planned)

**Trade-offs**:
- Precise GC requires statepoint overhead
- Type safety prevents some optimizations
- Dynamic arrays have growth overhead

## Future Enhancements

**Planned Features**:
1. **Generational GC**: Reduce GC pause times
2. **Concurrency**: Goroutine-style concurrency
3. **Generics**: Parametric polymorphism
4. **Pattern Matching**: Enhanced control flow
5. **Standard Library**: Expanded built-ins
6. **Incremental Compilation**: Faster builds
7. **JIT Compilation**: LLVM ORC JIT integration

## References

### Key Files

**Compiler**:
- `internal/lexer/lexer.go` - Lexical analysis
- `internal/parser/parser.go` - Syntax analysis
- `internal/ir/ir.go` - IR generation
- `internal/ir/tc.go` - Type checking
- `internal/codegen/codegen.go` - LLVM code generation
- `internal/zeus_compiler/compiler.go` - Compilation orchestration

**Runtime**:
- `runtime/gc_runtime.zig` - GC entry points
- `runtime/gc.zig` - GC implementation
- `runtime/array_runtime.zig` - Array operations
- `runtime/abi.zig` - Runtime ABI definitions
- `runtime/stackmap.zig` - Stack map parsing

**Build**:
- `makefile` - Primary build system
- `runtime/build.zig` - Runtime build
- `go.mod` - Go dependencies

### External Dependencies

**Go Packages**:
- `github.com/spf13/cobra` - CLI framework
- `tinygo.org/x/go-llvm` - LLVM Go bindings

**System Libraries**:
- LLVM (code generation and optimization)
- libunwind (stack unwinding)
- libc (standard C library)

---

**Document Version**: 1.1  
**Last Updated**: 2025-01-05  
**Zeus Version**: Development
**Major Changes**: Comprehensive array implementation documentation added

