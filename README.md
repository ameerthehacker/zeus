# Zeus

Zeus is a modern, garbage-collected programming language inspired by TypeScript, designed with a focus on simplicity and a JavaScript-like development experience. It combines the familiar syntax and semantics of TypeScript with automatic memory management, making it ideal for developers who want the productivity of JavaScript/TypeScript without manual memory management concerns.

## Key Features

- **TypeScript-inspired syntax** - Familiar language constructs for web developers
- **Automatic garbage collection** - No manual memory management required
- **JavaScript-like semantics** - Intuitive behavior and type system
- **Static typing** - Catch errors at compile time while maintaining ease of use
- **IDE support** - Built-in Language Server with real-time diagnostics and VS Code extension
- **Simple and clean** - Minimal complexity, maximum productivity

> [!WARNING]  
> This project is in early development and not ready for production use. The language syntax, features, and implementation are subject to significant changes.

## Getting Started

### Prerequisites

Before building Zeus, you'll need to install the following tools on macOS:

#### 1. Xcode Command Line Tools
Required for the C/C++ compiler (clang) and linker:
```sh
xcode-select --install
```

#### 2. Homebrew
If you don't have Homebrew installed:
```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

#### 3. Go (1.23 or later)
Required for building the Zeus compiler:
```sh
brew install go
```

Verify installation:
```sh
go version  # Should show 1.23 or later
```

#### 4. Zig (0.14 or later)
Required for building the Zeus runtime:
```sh
brew install zig
```

Verify installation:
```sh
zig version  # Should show 0.14 or later
```

#### 5. LLVM
The Zeus compiler uses LLVM bindings (installed via Go modules automatically):
```sh
# LLVM will be installed as a dependency when you build Zeus
# No manual installation needed for LLVM
```

### Building Zeus

1. **Clone the repository**:
   ```sh
   git clone https://github.com/ameerthehacker/zeus.git
   cd zeus
   ```

2. **Install Go dependencies**:
   ```sh
   go mod download
   ```

3. **Build the runtime**:
   ```sh
   make build-runtime
   ```

4. **Build the Zeus compiler** (optional, for using the `zeus` command):
   ```sh
   make build
   ```

5. **Verify installation**:
   ```sh
   # Run the test suite
   make test
   
   # Try compiling an example
   make run file=main
   ```

### Quick Start

Once everything is set up, try running the example in `playground/main.zs`:
```sh
make run file=main
```

You should see the program execute successfully!

**For the best development experience**, set up the VS Code extension to get real-time error checking and diagnostics. See the [IDE Support](#ide-support) section below.

## Language

Below is an example

```ts
// math.zs
// export class
export class Point {
  public x: i32;
  public y: i8;

  constructor(x: i32, y: i8) {
    this.x = x;
    this.y = y;
  }

  public sum(): i32 {
    return this.x + this.y;
  }
}
```

```ts
// main.zs
// import the class
import {Point} from "./math.zs";

function main(): i32 {
  let point: Point = new Point(1, 2);

  return point.sum();
}
```

## IDE Support

### Language Server Protocol (LSP)

Zeus includes a built-in Language Server Protocol implementation that provides real-time IDE features:

#### ✨ Available Features

- **Real-time Diagnostics** - Instant error and warning feedback as you type
  - Lexer errors (syntax errors)
  - Parser errors (structural issues)
  - Type errors (type mismatches, incompatible assignments)
  - Unused variable/function warnings
- **Syntax Highlighting** - Full syntax highlighting for `.zs` files
- **Smart Error Positioning** - Precise error underlining with detailed messages

#### 🚧 Planned Features (Coming Soon)

- **Autocompletion** - Keyword and context-aware suggestions
- **Hover Information** - Type information and documentation
- **Go to Definition** - Navigate to symbol definitions
- **Document Symbols** - Outline view of your code
- **Rename Symbol** - Rename across files
- **Format Document** - Automatic code formatting

#### Using with VS Code

1. **Build the Zeus compiler**:
   ```sh
   # Build zeus executable
   make build
   
   # Or manually:
   go build -o zeus zeus.go
   ```

2. **Install the Zeus VS Code extension** (development mode):
   ```sh
   cd zeus-vscode
   npm install
   npm run compile
   ```

3. **Configure the extension** (choose one option):

   **Option A: Add Zeus to PATH** (recommended)
   ```sh
   # Add to your ~/.zshrc or ~/.bashrc
   export PATH="$HOME/Projects/zeus:$PATH"
   
   # Then launch VS Code from terminal
   cd /path/to/your/project
   code .
   ```

   **Option B: Configure executable path in VS Code**
   
   Open VS Code Settings (`Cmd+,` on macOS, `Ctrl+,` on Windows/Linux) and search for "zeus", then set:
   ```json
   {
     "zeus.executablePath": "/path/to/zeus/zeus"
   }
   ```
   
   Or add to your workspace `.vscode/settings.json`:
   ```json
   {
     "zeus.executablePath": "${workspaceFolder}/zeus"
   }
   ```

4. **Run the extension**:
   - Press F5 in the `zeus-vscode/` folder to launch Extension Development Host
   - Or install the extension: `cd zeus-vscode && npm run vsix`
   - Open any `.zs` file to activate the language server

The extension automatically detects the Zeus executable in the following order:
1. Custom path from `zeus.executablePath` setting
2. `./zeus` in workspace root
3. `zeus` from system PATH

#### Standalone LSP Server

You can also run the LSP server standalone for integration with other editors:

```sh
# Build the zeus binary
make build

# Start the LSP server (communicates via stdio)
make lsp

# Or run directly:
./zeus lsp --stdio
```

The LSP server provides the Language Server Protocol interface over standard input/output, making it compatible with any editor that supports LSP.

## Usage

### Quick Reference: Makefile Commands

```sh
# Build commands
make build                    # Build the zeus compiler binary
make build-runtime           # Build the Zeus runtime
make build-zeus-vscode       # Package the VS Code extension

# Development commands
make run file=<name>         # Compile and run a playground file
make compile file=<name>     # Compile a playground file (no run)
make lsp                     # Start the Language Server

# Testing commands
make test                    # Run all tests (Go + runtime)
make test-go                 # Run Go tests only
make test-runtime            # Run runtime tests only
make test-e2e                # Run end-to-end tests

# Cleanup
make clean                   # Clean build artifacts
```

**Build flags** (for `run` and `compile`):
- `debug=true` - Enable debug output and Zeus IR
- `nogc=true` - Disable garbage collection
- `release=true` - Use optimized runtime build

### Building Zeus Programs

To compile a Zeus file use the following commands:

```sh
# emit executable file
go run zeus.go build main.zs

# emit object file
go run zeus.go build --file-type=obj main.zs

# emit readable llvm file
go run zeus.go build --file-type=ll main.zs

# emit assembly file
go run zeus.go build --file-type=asm main.zs
```

### Development with Playground

For quick testing and experimentation, use the `playground/` directory:

1. **Create a playground file**:
   ```sh
   # Create a new file in the playground directory
   touch playground/my_test.zs
   ```

2. **Write your Zeus code**:
   ```ts
   // playground/my_test.zs
   function main(): i32 {
     let x: i32 = 42;
     return x;
   }
   ```

3. **Compile and run**:
   ```sh
   # Compile and run (optimized build)
   make run file=my_test
   
   # Compile and run with debug output
   make run file=my_test debug=true
   
   # Just compile without running
   make compile file=my_test
   
   # Compile with release optimizations
   make compile-release file=my_test
   
   # Compile without garbage collection
   make run file=my_test nogc=true
   
   # Combine flags
   make run file=my_test debug=true nogc=true
   ```

The compiled binary will be placed in `playground/debug/my_test` and can be run directly:
```sh
./playground/debug/my_test
echo $?  # Print exit code
```

**Build flags**:
- `debug=true` - Enables Zeus IR output, LLVM IR files in `playground/debug/out/`, and GC debug logging
- `nogc=true` - Disables garbage collection (no GC passes, useful for performance comparison)
- `release=true` - Uses optimized runtime build (smaller binary size)

## Roadmap

### Beta V1
- [x] Tokenizer
- [x] Parser
- [x] Zeus IR
- [x] Type Checker
- [x] LLVM codegen
- [x] Scalar types
- [x] Class
- [x] GC v1
- [x] Arrays
- [ ] Strings
- [ ] Var Args
- [ ] Log Functions
- [ ] Inheritance
- [ ] Interfaces
- [ ] Variable Type Inference
- [ ] Function Type Inference
- [ ] Closure
- [ ] String Type
- [ ] Exception Handling
- [x] Language Server v1 (Diagnostics, Syntax Highlighting)
- [ ] Language Server v2 (Completions, Go to Definition, Hover)
- [ ] HTTP Server v1
- [ ] Standard Lib v1
- [ ] Release build mode
- [ ] Linux Support
- [ ] Package for Mac and Linux
- [ ] Docs site

### Beta V2
- [ ] Nullable Type