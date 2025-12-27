# Zeus

Zeus is a modern, garbage-collected programming language inspired by TypeScript, designed with a focus on simplicity and a JavaScript-like development experience. It combines the familiar syntax and semantics of TypeScript with automatic memory management, making it ideal for developers who want the productivity of JavaScript/TypeScript without manual memory management concerns.

## Key Features

- **TypeScript-inspired syntax** - Familiar language constructs for web developers
- **Automatic garbage collection** - No manual memory management required
- **JavaScript-like semantics** - Intuitive behavior and type system
- **Simple and clean** - Minimal complexity, maximum productivity
- **Static typing** - Catch errors at compile time while maintaining ease of use

> [!WARNING]  
> This project is in early development and not ready for production use. The language syntax, features, and implementation are subject to significant changes.

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

## Usage

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
- [ ] Language Server v1
- [ ] HTTP Server v1
- [ ] Standard Lib v1
- [ ] Release build mode
- [ ] Linux Support
- [ ] Package for Mac and Linux
- [ ] Docs site

### Beta V2
- [ ] Nullable Type