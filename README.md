<p align="center">
  <img src="assets/gsx-logo.svg" alt="GSX logo" width="420" />
</p>

<p align="center">
  <strong>Write HTML inline in ordinary Go functions.</strong><br />
  No template language, no runtime — just Go you can read.
</p>

<p align="center">
  <a href="https://kilianc.github.io/gsx/">Documentation</a> ·
  <a href="https://kilianc.github.io/gsx/language.html">Language reference</a> ·
  <a href="https://kilianc.github.io/gsx/composition.html">Components</a> ·
  <a href="https://kilianc.github.io/gsx/live-reload.html">Live reload</a>
</p>

> [!WARNING]
> mostly AI generated, not used in production yet, I would not use this if I were you

---

## Start simple

A `.gsx` file is a normal Go file. The only new thing is that a tag is an expression.

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

```html
<main class="page"><h1>Hello</h1><p>Welcome to GSX.</p></main>
```

## Then mix in Go

Because markup is an expression, everything you already do in Go still works. Loops build
lists. Conditionals pick branches. Values come from variables and function calls.

There is no template language between you and the page — no `{{range}}`, no partials to
register, no separate file to keep in sync.

```go
func ProfileCard(name string, tags []string, admin bool) Node {
  var lis []Node
  for _, t := range tags {
    lis = append(lis, <li class="tag">{t}</li>)
  }

  return (
    <section class="card">
      <header>
        <h2>{strings.TrimSpace(name)}</h2>
        {If(admin, <span class="pill">admin</span>)}
      </header>
      <ul class="tags">{lis}</ul>
    </section>
  )
}
```

Props are function parameters, so a typo is a compile error and your editor completes them
from the signature.

## And you can read the output

GSX runs ahead of time and writes a `.gsx.go` file next to each source file. You check it
in and review it like any other code.

```go
func ProfileCard(name string, tags []string, admin bool) Node {
	var lis []Node
	for _, t := range tags {
		lis = append(lis, html.Li(html.Class("tag"), Text(t)))
	}

	return html.Section(
		html.Class("card"),
		html.Header(
			html.H2(Text(strings.TrimSpace(name))),
			If(admin, html.Span(html.Class("pill"), Text("admin"))),
		),
		html.Ul(html.Class("tags"), Group(lis)),
	)
}
```

That is the whole trick. When something renders wrong, you open the generated file and read
exactly what will run — no reflection, no template parser, no interpreter at run time.
Rendering is [`maragu.dev/gomponents`](https://pkg.go.dev/maragu.dev/gomponents).

## Install

```bash
go install github.com/kilianc/gsx/cmd/gsx@latest
```

## Use

```bash
gsx ./...          # generate a .gsx.go next to every .gsx
gsx -check ./...   # CI: fail if any generated file is stale
gsx dev            # watch, rebuild, restart, reload the browser
gsx lsp            # language server (started by the editor extension)
```

## Live reload

```bash
gsx dev
```

Open <http://localhost:8080>. On every save GSX regenerates, restarts your app, waits for
it to accept connections, and reloads the browser.

That last part matters: a Go server has to be rebuilt and restarted before a refresh shows
anything new, so `gsx dev` supervises your app rather than only watching files.

**Your application needs no changes.** The reload client is injected by a proxy on the way
through, so nothing in your code refers to GSX and a production build cannot ship a dev
client. Build failures appear as a full-page overlay with the same `file:line:col` message
and source snippet you get on the terminal.

## What the syntax covers

| | |
|---|---|
| Tags | `<div class="x">…</div>`, `<input />` |
| Fragments | `<>…</>` |
| Splices | `{expr}` — a `string`, `Node`, or `[]Node` |
| Components | `<Card>`, `<ui.Card>`, `<ui.widgets.Card>` |
| Typed props | `<Badge text="Active" variant="success" />` |
| Comments | `{/* dropped at compile time */}` |
| JSX attributes | `className`, `htmlFor`, `maxLength`, … |
| Spread | `<span {...attrs}>` |
| Conditional attributes | `<input disabled={locked} />` |
| Entities | `&amp;`, `&#65;`, `&eacute;` decoded at compile time |

Text follows JSX's whitespace rules, so indented markup renders the way it reads.

Full details in the [language reference](https://kilianc.github.io/gsx/language.html).

## Editors

`gsx lsp` proxies `gopls`, compiling the buffer to a virtual Go view and mapping positions
back, so `.gsx` files get diagnostics, hover, go-to-definition and completion. GSX's own
parse errors are reported on the offending character.

A VS Code / Cursor extension lives in [`vscode/gsx-vscode/`](vscode/gsx-vscode). See the
[editor setup guide](https://kilianc.github.io/gsx/editors.html).

## Embedding the compiler

```go
import "github.com/kilianc/gsx/pkg/gsx"

out, err := gsx.CompileFile("page.gsx", src)
```

The internal compiler under `internal/gsx/...` is not part of the public API.

## Developing

```bash
make ci       # check generated files, vet, test
make golden   # regenerate goldens after a compiler change, then test
make docs     # build the documentation site into docs/dist
```

The `e2e/` package uses strict golden tests. The generated `*.gsx.go` **is** the golden —
it is compiled into the test binary, so a golden that does not build fails the package
build. `*.html.out` holds the expected rendered HTML for fixtures that register themselves
in `GSXFunctions`.

After a compiler change, run `make golden` and read the diff. That diff is the review
surface.

The [documentation site](https://kilianc.github.io/gsx/) is itself written in GSX
([`docs/`](docs)), and every code sample on it is compiled by the version of GSX that built
the page — so the docs cannot drift from the compiler.
