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

<p align="center">
  <img src="assets/gsx-demo.gif" alt="Typing a .gsx file, the generated .gsx.go appearing beside it, and the page reloading" width="920" />
</p>

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

**The CLI** (needs Go 1.22+):

```bash
go install github.com/kilianc/gsx/cmd/gsx@latest
```

The binary lands in `$(go env GOPATH)/bin`, which must be on your `PATH`. Check it with
`gsx -h`.

**The editor extension** is not on the Marketplace yet, so build it from the repository.
This needs [Docker](https://www.docker.com) — the Node toolchain is pinned in a container,
so nothing is installed on your machine:

```bash
git clone https://github.com/kilianc/gsx
cd gsx
make vsix
code --install-extension vscode/gsx-vscode/gsx-vscode-*.vsix
```

For Cursor, use `cursor --install-extension` instead. If you already have Node and would
rather not use Docker, `cd vscode/gsx-vscode && npm install && npm run compile && npx vsce
package --no-dependencies` does the same thing.

The extension runs the `gsx` binary, so install the CLI first either way.

## Use

```bash
gsx ./...          # generate a .gsx.go next to every .gsx
gsx -check ./...   # CI: fail if any generated file is stale
gsx fmt -w ./...   # format sources
gsx fmt -l ./...   # CI: fail if any source is unformatted
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
back, so `.gsx` files get diagnostics, hover and go-to-definition. GSX's own parse errors are
reported on the offending character.

Requests gopls cannot answer — it has no notion of a tag — GSX answers itself: tag completion
after `<` (your components first, then HTML elements), attribute completion inside a start
tag, closing tags after `</`, and formatting.

The VS Code / Cursor extension in [`vscode/gsx-vscode/`](vscode/gsx-vscode) adds the editing
behaviour a JSX-like language needs:

- **Auto-close tags** — typing `>` inserts the closing tag, leaving the caret between them
- **Linked editing** — renaming an opening tag renames its closing tag as you type
- **Syntax highlighting** that knows the Go/markup boundary: components colour differently
  from elements, prose is not highlighted as Go, and `a < b` is never mistaken for a tag
- **Snippets** — `comp`, `layout`, `frag`, `each`, `if`
- **Format on save** — Go formatted as gofmt would, markup re-indented but not reflowed
- **Commands** — generate, dev server, and open the generated `.gsx.go` side by side

See the [editor setup guide](https://kilianc.github.io/gsx/editors.html).

## Embedding the compiler

```go
import "github.com/kilianc/gsx/pkg/gsx"

out, err := gsx.CompileFile("page.gsx", src)
```

The internal compiler under `internal/gsx/...` is not part of the public API.

## Developing

```bash
make ci            # check generated files, vet, test
make golden        # regenerate goldens after a compiler change, then test
make docs          # build the documentation site into docs/dist
make ci-extension  # grammar + extension tests, typecheck
make vsix          # package the editor extension
```

The extension is the only part of this repository that needs Node, and it never touches your
machine: the toolchain is pinned in [`tools/Dockerfile`](tools/Dockerfile) and every
extension target runs in that container.

A TextMate grammar cannot be verified by reading it, so `make grammar-test` tokenizes
fixtures with the same engine VS Code uses and asserts on the resulting scopes — including
that Go's `a < b`, `a << 2` and `<-ch` never scope as tags.

The `e2e/` package uses strict golden tests. The generated `*.gsx.go` **is** the golden —
it is compiled into the test binary, so a golden that does not build fails the package
build. `*.html.out` holds the expected rendered HTML for fixtures that register themselves
in `GSXFunctions`.

After a compiler change, run `make golden` and read the diff. That diff is the review
surface.

The [documentation site](https://kilianc.github.io/gsx/) is itself written in GSX
([`docs/`](docs)), and every code sample on it is compiled by the version of GSX that built
the page — so the docs cannot drift from the compiler.
