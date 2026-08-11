package site

// example is what the playground opens with. It is deliberately a little more
// than "hello": a loop, a conditional and a splice calling into Go are the
// things a reader is actually trying to find out about.
const example = `package main

import "fmt"

type Product struct {
	Name  string
	Price float64
	Sale  bool
}

// Page is the entry point. The playground renders whatever it returns.
func Page() Node {
	products := []Product{
		{"Keyboard", 89.99, true},
		{"Monitor", 329.50, false},
		{"Desk mat", 24.00, false},
	}

	var rows []Node
	for _, p := range products {
		class := "row"
		if p.Sale {
			class += " on-sale"
		}

		rows = append(rows, (
			<tr class={class}>
				<td>{p.Name}</td>
				<td>{fmt.Sprintf("$%.2f", p.Price)}</td>
				<td>{badge(p.Sale)}</td>
			</tr>
		))
	}

	return (
		<table class="cart">
			<thead>
				<tr><th>Item</th><th>Price</th><th></th></tr>
			</thead>
			<tbody>{rows}</tbody>
		</table>
	)
}

func badge(sale bool) Node {
	if !sale {
		return nil
	}
	return <span class="badge">Sale</span>
}
`

func PlaygroundPage() Page {
	return Page{
		Slug:     "playground",
		Title:    "Playground",
		Subtitle: "Write GSX on the left. The Go it compiles to, and the HTML that Go renders, appear on the right.",
		Wide:     true,
		Scripts:  []string{"./playground.js"},
		Body: (
			<>
				<div class="pg">
					<div class="pg-col">
						<div class="pg-bar">
							<span class="pg-label">page.gsx</span>
							<span id="pg-status" class="pg-status is-busy">Loading compiler…</span>
						</div>
						{El("textarea", Attr("id", "pg-editor"), Attr("class", "pg-editor"),
							Attr("spellcheck", "false"), Attr("autocomplete", "off"),
							Attr("autocapitalize", "off"), Attr("aria-label", "GSX source"),
							Text(example))}
					</div>

					<div class="pg-col">
						<div class="pg-bar">
							<button class="pg-tab is-active" data-pane="preview" type="button">Preview</button>
							<button class="pg-tab" data-pane="go" type="button">Generated Go</button>
							<button class="pg-tab" data-pane="html" type="button">HTML</button>
						</div>

						<div id="pg-error" class="pg-error" hidden></div>

						<div class="pg-body" data-pane-body="preview">
							{El("iframe", Attr("id", "pg-preview"), Attr("class", "pg-preview"),
								Attr("title", "Rendered output"), Attr("sandbox", ""))}
						</div>
						<div class="pg-body" data-pane-body="go" hidden>
							<pre class="pg-out"><code id="pg-go"></code></pre>
						</div>
						<div class="pg-body" data-pane-body="html" hidden>
							<pre class="pg-out"><code id="pg-html"></code></pre>
						</div>
					</div>
				</div>

				<div class="pg-note">
					<p>
						Everything here runs in your browser. The compiler is the same one the
						{" "}<code>gsx</code> command uses, built to WebAssembly; the generated Go is
						interpreted rather than built, so no code leaves the page and there is no
						server to send it to.
					</p>
					<p class="muted">
						The interpreter can see {" "}<code>gomponents</code>, <code>fmt</code>,
						{" "}<code>strings</code>, <code>strconv</code> and <code>sort</code>. It cannot
						see <code>os</code> or <code>net</code> — those packages are simply absent, which
						is what makes running a stranger's snippet safe.
					</p>
				</div>
			</>
		),
	}
}
