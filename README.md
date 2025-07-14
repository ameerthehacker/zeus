# Zeus

TypeScript like compiled language

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
