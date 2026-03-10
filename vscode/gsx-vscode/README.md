# GSX - Go + HTML Components

GSX lets you write HTML components directly inside Go files using a JSX-like syntax. The compiler transforms `.gsx` files into standard Go code powered by [gomponents](https://www.gomponents.com). This extension provides full language support for `.gsx` files in VS Code and Cursor.

## Features

- **Syntax highlighting** for Go code with embedded HTML tags
- **Diagnostics** (errors and warnings) mapped back to `.gsx` source positions
- **Go to definition** for Go symbols inside and outside of HTML expressions
- **Hover** information for types, functions, and variables
- **Completions** powered by gopls
- **File nesting** automatically groups `.gsx.go` files under their `.gsx` source

## Prerequisites

1. **gsx** compiler installed and on your PATH:

   ```bash
   go install github.com/kilianc/gsx/cmd/gsx@latest
   ```

2. **gopls** (Go language server) installed:

   ```bash
   go install golang.org/x/tools/gopls@latest
   ```

3. **Go extension** for VS Code (`golang.go`) must be installed.

## Configuration

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `gsx.executablePath` | string | `""` | Path to the `gsx` binary. Set this if the extension cannot find it automatically. |
| `gsx.goplsLog` | string | `""` | File path to write gopls logs to, for debugging. |
| `gsx.goplsRPCTrace` | boolean | `false` | Enable verbose gopls RPC tracing. |
| `gsx.goplsRemote` | string | `""` | Connect to a remote gopls instance instead of spawning one locally. |

## How It Works

The extension starts a `gsx lsp` process that acts as an LSP proxy between your editor and `gopls`. It compiles `.gsx` files on the fly into virtual Go files, forwards them to `gopls` for analysis, and maps the results (diagnostics, definitions, hover, completions) back to the original `.gsx` source positions.

## Known Limitations

- Incremental document sync is not supported; every edit sends the full file content.
- Inlay hints, code actions, and semantic tokens from gopls are disabled to avoid conflicts with the Go extension.
- Multi-line expressions inside `{...}` may not map correctly for go-to-definition.
