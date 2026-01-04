# Zeus Language Support for VS Code

Language support for the Zeus programming language.

## Features

- **Syntax Highlighting**: Full syntax highlighting for `.zs` files
- **Language Server Protocol (LSP)**: Integrated language server with:
  - Real-time error diagnostics (lexer and parser errors)
  - Syntax validation
  - Error highlighting with hover details

## Requirements

The Zeus compiler/language server must be installed and accessible in your PATH.

### Installation

```bash
git clone https://github.com/ameerthehacker/zeus
cd zeus
go build -o zeus zeus.go

# Add zeus to your PATH or use the configuration below
export PATH="$HOME/Projects/zeus:$PATH"
```

## Configuration

- `zeus.executablePath`: (Optional) Custom path to the Zeus executable. If not set, uses `zeus` from PATH.

### Example Configuration

If Zeus is not in your PATH, you can configure it in VS Code settings:

```json
{
  "zeus.executablePath": "/Users/yourname/Projects/zeus/zeus"
}
```

## Features in Detail

### Diagnostics

The extension provides real-time error detection as you type:
- **Syntax Errors**: Identifies and highlights syntax errors
- **Parser Errors**: Shows parsing issues with detailed messages
- **Error Severity**: Differentiates between errors, warnings, and info messages

### Troubleshooting

#### "spawn zeus ENOENT" error

The extension can't find the `zeus` executable. Solutions:

1. **Add Zeus to PATH** (recommended):
   - Add to your `.zshrc` or `.bashrc`: `export PATH="$HOME/Projects/zeus:$PATH"`
   - Launch VS Code from terminal: `code .`

2. **Configure the path**: Set `zeus.executablePath` in VS Code settings

## Development

To test the extension locally:

1. Build Zeus: `go build -o zeus zeus.go`
2. Build the extension: `cd zeus-vscode && npm run compile`
3. Press F5 in VS Code to launch the Extension Development Host
4. Open a `.zs` file to see diagnostics in action

## Release Notes

### 0.0.1

- Initial release
- Syntax highlighting for Zeus files
- Language Server Protocol integration
- Real-time diagnostics (lexer and parser errors)

---

**Enjoy!**
