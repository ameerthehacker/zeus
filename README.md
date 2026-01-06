# Zeus ⚡

**TypeScript's Soul. Native Speed.**

Zeus is a modern programming language that combines TypeScript's familiar syntax with native compilation. Write code that feels like TypeScript, but compiles to blazing-fast native binaries.

[![Documentation](https://img.shields.io/badge/docs-zeus--lang.vercel.app-blue)](https://zeus-lang.vercel.app/)
[![VS Code](https://img.shields.io/badge/VS%20Code-Extension-007ACC?logo=visual-studio-code)](https://marketplace.visualstudio.com/items?itemName=ameerthehacker.zeus-vscode)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> ⚠️ **Early Development** — Zeus is evolving rapidly. Not recommended for production use yet.

## Why Zeus?

| Feature | Description |
|---------|-------------|
| 🎯 **Familiar Syntax** | If you know TypeScript, you already know 80% of Zeus |
| 🚀 **Native Speed** | LLVM-powered compilation to machine code — no interpreter |
| 🧹 **Automatic GC** | Focus on logic, not memory management |
| 💡 **IDE Ready** | VS Code extension with real-time diagnostics |

## Installation

```bash
brew tap ameerthehacker/zeus https://github.com/ameerthehacker/zeus
brew install zeus
```

## Quick Example

```typescript
class Point {
  public x: i32;
  public y: i32;

  constructor(x: i32, y: i32) {
    this.x = x;
    this.y = y;
  }

  public sum(): i32 {
    return this.x + this.y;
  }
}

function main(): i32 {
  let p: Point = new Point(10, 20);
  return p.sum();  // Returns 30
}
```

```bash
zeus build main.zs -o main
./main
```

## Documentation

📖 **[zeus-lang.vercel.app](https://zeus-lang.vercel.app/)** — Full documentation, tutorials, and language reference.

- [Getting Started](https://zeus-lang.vercel.app/getting-started/installation/)
- [Language Guide](https://zeus-lang.vercel.app/language/variables/)
- [IDE Setup](https://zeus-lang.vercel.app/getting-started/ide-setup/)
- [Examples](https://zeus-lang.vercel.app/examples/fibonacci/)

## VS Code Extension

Get syntax highlighting and real-time diagnostics — **[Install from Marketplace →](https://marketplace.visualstudio.com/items?itemName=ameerthehacker.zeus-vscode)**

The extension auto-detects Homebrew installations — no configuration needed!

## License

MIT
