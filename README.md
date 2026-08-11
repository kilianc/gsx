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

**Verify generated files are up to date** (writes nothing, exits non-zero if any are stale):

```bash
gsx -check ./...
```

Use this in CI so a `.gsx` edit can never land without its regenerated `.gsx.go`.

## GSX syntax

`*.gsx` files are **Go code** with one extra expression form:

- **Tag expressions**: `<tag ...attrs...> ...children... </tag>` and self-closing `<input ... />`
- **Fragments**: `<>...</>` groups siblings without emitting a wrapper element
- **Go expression splices**: `{expr}` inside children or attribute positions
  - Child `{expr}` must be a `string`, a `Node`, or a `[]Node` (slices are auto-wrapped as `Group(slice)`).
  - Attribute expressions must typecheck as expected by gomponents helpers (e.g. `class={s}` becomes `Class(s)`).
- **Comments**: `{/* ... */}` is dropped at compile time, in both child and attribute position

### Attributes

Attribute names accept both the HTML spelling and the **JSX spelling**, so pasted JSX
compiles and muscle memory doesn't produce a silently wrong attribute:

```go
<label htmlFor="email">Email</label>          // → for="email"
<div className="card">…</div>                 // → class="card"
<input autoComplete="off" maxLength="120" />  // → autocomplete, maxlength
```

Names are matched case-insensitively, and `data-*` / `aria-*` pass through as written.

**Spread attributes** apply a prebuilt `[]Node` of attributes to an element:

```go
func sharedAttrs() []Node {
  return []Node{Attr("class", "shared"), Attr("data-kind", "demo")}
}

<span {...sharedAttrs()}>one</span>
<button type="button" {...sharedAttrs()} disabled>two</button>
```

> Inside plain Go — a helper like `sharedAttrs` above — `gomponents/html` helpers need an
> explicit `html.` prefix (`html.Class("x")`). Bare names are only resolved inside tag
> expressions and `{...}` splices.

### Fragments

Two sibling tags can't sit adjacent in one Go expression — but a fragment can wrap them:

```go
func Header() Node {
  return (
    <>
      <h1>Title</h1>
      <p>Subtitle</p>
    </>
  )
}
```

This compiles to `Group{html.H1(...), html.P(...)}`, which renders both children with no
enclosing element.

### Text, whitespace and entities

Literal text follows **JSX's rules**, so markup renders the way it reads:

```go
<p>
  This sentence is written
  across three source lines
  but renders as one.
</p>
```

```html
<p>This sentence is written across three source lines but renders as one.</p>
```

Concretely: a run of text containing a line break has each line trimmed, blank lines
dropped, and the rest joined with a single space. A run **without** a line break is kept
byte for byte, so inline spacing survives:

```go
<p><b>bold</b> then <i>italic</i></p>   // the spaces around "then" are preserved
```

**HTML entities are decoded at compile time**, then escaped again on render — so what you
write is what the reader sees:

```go
<p>Tom &amp; Jerry &lt;3 caf&eacute;</p>
```

```html
<p>Tom &amp; Jerry &lt;3 café</p>
```

Only well-formed references (`&name;`, `&#65;`, `&#x41;`) are decoded. Unlike a browser,
GSX will not decode a reference missing its semicolon, so `?page=1&next=2` and `AT&T`
survive untouched.

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

**Dotted tags** call qualified functions from imported packages, including nested ones
(`<ui.widgets.Card>`):

```go
import "myapp/ui"

func Page() Node {
  return <ui.Card><p>Hello</p></ui.Card>
}
```

**Convention**: lowercase tags (`<div>`, `<span>`) are always HTML elements. Uppercase tags (`<Card>`, `<ui.Card>`) are always component function calls.

### Typed props

Components can accept **typed parameters** (not just `...Node` children). Attributes are mapped to function parameters by name:

```go
func SectionHeading(text string) Node {
  return <h2>{text}</h2>
}

func Badge(text string, variant string) Node {
  return <span class={"badge badge-" + variant}>{text}</span>
}
```

Use them just like JSX props:

```go
<SectionHeading text="Debug" />
<Badge text="Active" variant="success" />
```

This compiles to `SectionHeading("Debug")` and `Badge("Active", "success")`.

Any Go type works as a prop — `time.Time`, `sql.NullString`, custom structs, pointers, slices, etc.:

```go
import "time"

func CellTime(t time.Time) Node {
  return <td>{t.Format(time.RFC3339)}</td>
}
```

```go
<CellTime t={row.CreatedAt} />
// compiles to: CellTime(row.CreatedAt)
```

**Mixed props and children** work too — typed parameters come from attributes, `...Node` children come from the body:

```go
func SectionWithHeading(heading string, children ...Node) Node {
  return <section><h2>{heading}</h2>{children}</section>
}
```

```go
<SectionWithHeading heading="Tasks">
  <p>content here</p>
</SectionWithHeading>
// compiles to: SectionWithHeading("Tasks", P(Text("content here")))
```

### Notes

- Components are normal Go `func`s and must have an explicit `return ...`.
- You can’t place two sibling tag expressions adjacent in one Go expression; wrap them in a
  parent tag (`return <div>{a}{b}</div>`) or a fragment (`return <>{a}{b}</>`).

## Public API

If you want to embed compilation in your own tooling, use `gsx.CompileFile`:

```go
import "github.com/kilianc/gsx/pkg/gsx"

out, err := gsx.CompileFile("page.gsx", src)
```

The internal compiler lives under `internal/gsx/...` and is not part of the public API.

## Tests

```bash
make ci
```

That runs three things:

1. `gsx -check ./...` — every checked-in `*.gsx.go` must match what the compiler produces right now.
2. `go vet ./...`
3. `go test ./...`

The `e2e/` package uses strict golden tests:

- **`*.gsx.go` is the golden generated Go.** It is not a separate copy of the expected
  output — it is the real generated file, and it is compiled into the test binary. A
  golden that does not build fails the package build.
- **`*.html.out` is the expected rendered HTML.** A fixture opts in by registering itself:

  ```go
  func init() {
    GSXFunctions["my_fixture"] = func() Node { return MyFixture() }
  }
  ```

After changing the compiler, refresh both kinds of golden with:

```bash
make golden
```

Then read the diff before committing — that diff *is* the review surface for a compiler change.

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
