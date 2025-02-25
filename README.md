# Zeus

TypeScript like compiled language

> [!WARNING]  
> This project is in early development and not ready for production use. The language syntax, features, and implementation are subject to significant changes.

## Language

Below is an example `main.zs` file

```ts
function main() {
  let i: i8 = 0;

  while (i < 10) {
    i = i + 1;
  }
  
  return i;
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
