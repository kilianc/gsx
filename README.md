<p align="center">
  <img src="assets/gsx-logo.svg" alt="GSX logo" width="420" />
</p>

# gsx

> [!WARNING]
> mostly AI generated, not used in production yet, I would not use this if I were you

**GSX** is “JSX-ish for Go”: write normal Go functions in `*.gsx`, with inline HTML-like tag expressions (`<div>...</div>`). Run `gsx` to generate checked-in, `gofmt`’d `*.gsx.go` files.

Under the hood, generated code leverages [`maragu.dev/gomponents`](https://pkg.go.dev/maragu.dev/gomponents) for HTML rendering.

Debugging is straightforward: the output is just a **well-formatted, human-readable Go file** (using gomponents), so when something looks off you can open the generated `*.gsx.go` and see exactly what will run.

## Example

Start simple: **pure markup** in a normal Go function.

**`hello.gsx`**

```go
package ui

func Hello() Node {
  return (
    <main class="page">
      <h1>Hello</h1>
      <p>Welcome to GSX.</p>
    </main>
  )
}
```

### Rendered HTML

```html
<main class="page"><h1>Hello</h1><p>Welcome to GSX.</p></main>
```

But the real power is mixing Go and markup like JSX. GSX is basically **“JSX for Go”**, without claiming 1:1 feature parity.

**`profile.gsx`**

```go
package ui

import "strings"

func ProfileCard(name string, tags []string, admin bool) Node {
  var lis []Node
  for _, t := range tags {
    lis = append(lis, <li class="tag">{t}</li>)
  }

  title := strings.TrimSpace(name)
  badge := ""
  if admin {
    badge = "admin"
  }

  top := (
    <header class={badge}>
      <h2>{title}</h2>
      {If(admin, <span class="pill">admin</span>)}
    </header>
  )

  bottom := <ul class="tags">{lis}</ul>

  return (
    <section class="card">
      <div>{top}{bottom}</div>
    </section>
  )
}
```

## Install

**Published CLI**:

```bash
go install github.com/kilianc/gsx/cmd/gsx@latest
```

## CLI usage

**Generate for the whole module**:

```bash
gsx ./...
```

**Generate for one directory (non-recursive)**:

```bash
gsx ./e2e
```

This writes `file.gsx.go` next to each `file.gsx`.

## GSX syntax

`*.gsx` files are **Go code** with one extra expression form:

- **Tag expressions**: `<tag ...attrs...> ...children... </tag>` and self-closing `<input ... />`
- **Go expression splices**: `{expr}` inside children or attribute positions
  - Child `{expr}` must be a `string`, a `Node`, or a `[]Node` (slices are auto-wrapped as `Group(slice)`).
  - Attribute expressions must typecheck as expected by gomponents helpers (e.g. `class={s}` becomes `Class(s)`).

### Components

Like JSX, **uppercase tags** invoke Go functions instead of emitting HTML elements:

```go
func Card(children ...Node) Node {
  return <div class="card">{children}</div>
}

func Page() Node {
  return <Card class="primary"><p>Hello</p></Card>
}
```

This generates `Card(Class("primary"), P(Text("Hello")))` — attributes and children are passed as `...Node` arguments, the same way gomponents HTML helpers work.

**Dotted tags** call qualified functions from imported packages:

```go
import "myapp/ui"

func Page() Node {
  return <ui.Card><p>Hello</p></ui.Card>
}
```

**Convention**: lowercase tags (`<div>`, `<span>`) are always HTML elements. Uppercase tags (`<Card>`, `<ui.Card>`) are always component function calls.

### Notes

- Components are normal Go `func`s and must have an explicit `return ...`.
- You can’t place two sibling tag expressions adjacent in one Go expression; wrap them in a parent tag (e.g. `return <div>{a}{b}</div>`).

## Public API

If you want to embed compilation in your own tooling, use `gsx.CompileFile`:

```go
import "github.com/kilianc/gsx/pkg/gsx"

out, err := gsx.CompileFile("page.gsx", src)
```

The internal compiler lives under `internal/gsx/...` and is not part of the public API.

## Tests

```bash
go test ./...
```

The `e2e/` package uses strict golden tests:

- `*.gsx.out` is expected generated Go
- `*.html.out` is expected rendered HTML for registered fixtures

## Editor setup (Cursor/VS Code)

### Quick + simple (highlighting only)

To treat `*.gsx` as Go in the editor (syntax highlighting), add:

```json
{
  "files.associations": {
    "*.gsx": "go"
  }
}
```

### Full IDE features (gopls-backed LSP)

GSX now includes a **`gsx lsp`** mode that proxies `gopls` and rewrites `*.gsx` to a virtual Go view so you get Go-like:

- diagnostics (lint/typecheck)
- completion
- hover
- go-to-definition

There’s also a thin VS Code extension under `vscode/gsx-vscode/` that runs `gsx lsp`.
