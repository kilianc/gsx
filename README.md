<p align="center">
  <img src="assets/gsx-logo.svg" alt="GSX logo" width="420" />
</p>

<p align="center">
  <strong>JSX for Go.</strong><br />
  Write markup inline in ordinary Go functions — with real loops, conditionals and
  calls in the middle of it.<br />
  No template language, no runtime.
</p>

<p align="center">
  <a href="https://kilianc.github.io/gsx/">Documentation</a> ·
  <a href="https://kilianc.github.io/gsx/playground.html">Playground</a> ·
  <a href="https://kilianc.github.io/gsx/language.html">Language reference</a> ·
  <a href="https://kilianc.github.io/gsx/composition.html">Components</a> ·
  <a href="https://kilianc.github.io/gsx/live-reload.html">Live reload</a>
</p>

> [!WARNING]
> mostly AI generated, not used in production yet, I would not use this if I were you

---

## If you know JSX, you already know GSX

A `.gsx` file is a normal Go file. The only new thing is the one JSX added to JavaScript:
**a tag is an expression.** Here is the same component in both languages.

```jsx
function ProfileCard({ name, tags, admin }) {
  return (
    <section className="card">
      <header>
        <h2>{name.trim()}</h2>
        {admin && <span className="pill">admin</span>}
      </header>
      <ul className="tags">
        {tags.map((t) => <li className="tag">{t}</li>)}
      </ul>
    </section>
  );
}
```

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

Same shape, same instincts. Tags, fragments, `{expr}` splices, `className`, spread
attributes and `{/* comments */}` all carry over. What changes is the language *around* the
tags: `map` becomes a `for` loop, `&&` becomes `If`, and props are typed function
parameters instead of an untyped object.

## Real Go, right in the middle of the markup

This is the whole point. Because a tag is an expression, markup sits inside ordinary Go
control flow, and Go control flow sits inside markup. Nothing is a special template
directive — it is the language you already write.

- `for` and `range` build lists into a `[]Node`.
- `if`, `switch` and early `return` pick between branches of markup.
- Function and method calls fill in values, and any function returning `Node` is a component.
- Variables hold markup, so you can build a piece of a page before you place it.

```go
func InvoiceTable(invoices []Invoice) Node {
  if len(invoices) == 0 {
    return <p class="empty">No invoices yet.</p>
  }

  var rows []Node
  for _, inv := range invoices {
    rows = append(rows, (
      <tr>
        <td>{inv.Number}</td>
        <td>{inv.Customer.Name}</td>
        <td class="num">{money(inv.Total)}</td>
        <td>{StatusPill(inv.Status)}</td>
      </tr>
    ))
  }

  return (
    <table class="invoices">
      <tbody>{rows}</tbody>
    </table>
  )
}

func StatusPill(s Status) Node {
  switch s {
  case Paid:
    return <span class="pill pill-ok">paid</span>
  case Overdue:
    return <span class="pill pill-bad">overdue</span>
  }
  return <span class="pill">draft</span>
}
```

There is no template language between you and the page — no `{{range}}`, no partials to
register, no separate file to keep in sync. The empty-state check is a plain `if` and the
status pill is a plain `switch`.

Props are function parameters, so a typo is a compile error, your editor completes them
from the signature, and a rename refactors every call site.

## Coming from JSX

| JSX | GSX |
|---|---|
| `<div>…</div>`, `<>…</>`, `{expr}` | identical |
| `className`, `htmlFor`, `maxLength` | identical (or the HTML spelling, your choice) |
| `{items.map(i => <li>{i}</li>)}` | a `for` loop appending to `[]Node`, spliced |
| `{cond && <p/>}` | `{If(cond, <p/>)}`, or plain Go `if` |
| `{...props}` | `{...attrs}`, where `attrs` is a `[]Node` |
| a props object | typed function parameters |
| `children` | a trailing `...Node` parameter |
| `<Card/>` resolved by scope | uppercase tags are function calls, resolved lexically |
| hooks, state, a virtual DOM | none of it — GSX only renders |

GSX is the JSX half of the deal: the syntax, on the server, ahead of time. For
interactivity, reach for whatever you would otherwise pair with server-rendered HTML.

## No runtime — the generated Go is the artifact

GSX runs ahead of time and writes a `.gsx.go` file next to each source file. You check it
in and review it like any other code. `InvoiceTable` above becomes:

```go
func InvoiceTable(invoices []Invoice) Node {
	if len(invoices) == 0 {
		return html.P(html.Class("empty"), Text("No invoices yet."))
	}

	var rows []Node
	for _, inv := range invoices {
		rows = append(rows, html.Tr(
			html.Td(Text(inv.Number)),
			html.Td(Text(inv.Customer.Name)),
			html.Td(html.Class("num"), Text(money(inv.Total))),
			html.Td(StatusPill(inv.Status)),
		))
	}

	return html.Table(html.Class("invoices"), html.TBody(Group(rows)))
}
```

The same function, with the tags spelled out. When something renders wrong you open that
file and read exactly what will run — no reflection, no template parser, no interpreter, no
diffing. At run time it is ordinary
[`maragu.dev/gomponents`](https://pkg.go.dev/maragu.dev/gomponents) calls you can step
through in a debugger, and nothing GSX-specific ships in your binary.

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

The JSX surface, with Go inside the braces:

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
