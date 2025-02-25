# Zeus

TypeScript like compiled language

⚠️ **Warning**: This project is in early development and not ready for production use. The language syntax, features, and implementation are subject to significant changes.

## Usage

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
