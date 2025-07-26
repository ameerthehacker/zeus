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

To compile a zeus file use the bellow commands

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

## Roadmap

### Beta
- [x] Tokenizer
- [x] Parser
- [x] Zeus IR
- [x] Type Checker
- [x] LLVM codegen
- [x] Scalar types
- [x] Class
- [x] GC v1
- [ ] Inheritance
- [ ] Interfaces
- [ ] Variable Type Inference
- [ ] Function Type Inference
- [ ] Closure
- [ ] String Type
- [ ] Arrays
- [ ] Union Type
- [ ] Match Expression
- [ ] Exception Handling
- [ ] Language Server v1
- [ ] HTTP Server v1
- [ ] Standard Lib v1
- [ ] Release build mode
- [ ] Linux Support
- [ ] Package for Mac and Linux
- [ ] Docs site