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

### Array Runtime

**Location**: `runtime/array_runtime.zig`

**Array Object**:
```zig
pub const ZeusArrayObj = extern struct {
  obj_header: *ZeusObjectHeader,
  length: i32,           // Current number of elements
  capacity: u32,         // Allocated capacity
  data: ?*anyopaque,     // Pointer to data buffer
};
```

**Operations**:
- **`zeus_array_constructor(this, return_buffer, capacity)`**:
  - Allocates initial data buffer
  - Initializes with default values
  
- **`zeus_array_push(this, return_buffer, value)`**:
  - Grows array if needed (2x growth factor)
  - Appends element to end
  
- **`zeus_array_pop(this, return_buffer)`**:
  - Returns and removes last element
  - Returns default value if empty
  
- **`zeus_array_get(this, return_buffer, index)`**:
  - Returns element at index
  - Bounds checking
  
- **`zeus_array_set(this, return_buffer, index, value)`**:
  - Sets element at index
  - Auto-grows array if necessary

**Memory Management**:
- Separate allocator for array data buffers
- GC calls `zeus_array_cleanup()` when freeing arrays
- Data buffers freed independently of object memory

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

**Document Version**: 1.0  
**Last Updated**: 2025-12-27  
**Zeus Version**: Development

