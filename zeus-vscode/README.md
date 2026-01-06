# Zeus Language Support for VS Code

Official VS Code extension for the [Zeus programming language](https://zeus-lang.vercel.app/) — TypeScript's soul with native speed.

## Features

- ✅ **Syntax Highlighting** — Full color coding for `.zs` files
- ✅ **Real-time Diagnostics** — Errors and warnings as you type
- ✅ **Code Completion** — Keywords, types, variables, and functions
- ✅ **Error Underlining** — Precise error locations with hover details

## Installation

### 1. Install Zeus

```bash
brew tap ameerthehacker/zeus https://github.com/ameerthehacker/zeus
brew install zeus
```

The extension **automatically detects** Homebrew installations — no configuration needed!

### 2. Install the Extension

**[Install from VS Code Marketplace →](https://marketplace.visualstudio.com/items?itemName=ameerthehacker.zeus-vscode)**

Or search for "Zeus" in the VS Code Extensions panel.

## Configuration

If you installed Zeus manually, configure the path in VS Code settings:

```json
{
  "zeus.executablePath": "/path/to/zeus"
}
```

## Commands

- **Zeus: Restart Language Server** — Restart the LSP if needed

## Learn More

- 📖 [Zeus Documentation](https://zeus-lang.vercel.app/)
- 🐙 [GitHub Repository](https://github.com/ameerthehacker/zeus)

---

**Enjoy building with Zeus! ⚡**
