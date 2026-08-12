package site

func IndexPage() Page {
	return Page{
		Slug:     "index",
		Title:    "GSX",
		Subtitle: "JSX for Go. Write markup inline in ordinary Go functions — with real loops, conditionals and calls in the middle of it. No template language, no runtime.",
		Body:     (
			<>
				<div class="hero-cta">
					<a class="btn btn-primary" href="./language.html">Read the language guide</a>
					<a class="btn" href="./playground.html">Try it in the playground</a>
					<a class="btn" href="https://github.com/kilianc/gsx">GitHub</a>
				</div>

				{Shell("go install github.com/kilianc/gsx/cmd/gsx@latest")}

				{Section("jsx", "If you know JSX, you already know GSX",
					P(Text("A "), Code(".gsx"), Text(" file is a normal Go file. The only new thing is the one JSX added to JavaScript: "),
						Strong("a tag is an expression"), Text(". Here is the same component in both languages.")),
					Split(
						Labeled("javascript · jsx", `
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
`),
						Labeled("go · gsx", `
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
`),
					),
					P(Text("Same shape, same instincts. Tags, fragments, "), Code("{expr}"), Text(" splices, "),
						Code("className"), Text(", spread attributes, "), Code("{/* comments */}"), Text(" — the syntax carries over.")),
					P(Text("What changes is the language "), Em("around"), Text(" the tags. "), Code("map"), Text(" becomes a "),
						Code("for"), Text(" loop, "), Code("&&"), Text(" becomes "), Code("If"),
						Text(", and props are typed function parameters instead of an untyped object.")),
				)}

				{Section("go", "Real Go, right in the middle of the markup",
					P(Text("This is the whole point. Because a tag is an expression, markup sits inside ordinary Go control flow, "),
						Text("and Go control flow sits inside markup. Nothing is a special template directive — it is the language you already write.")),
					<ul>
						<li><code>for</code> and <code>range</code> build lists into a <code>[]Node</code>.</li>
						<li><code>if</code>, <code>switch</code> and early <code>return</code> pick between branches of markup.</li>
						<li>Function and method calls fill in values, and any function returning <code>Node</code> is a component.</li>
						<li>Variables hold markup, so you can build a piece of a page before you place it.</li>
					</ul>,
					CompiledBelow(`
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
`),
					P(Text("There is no template language between you and the page — no "), Code("{{range}}"),
						Text(", no partials to register, no separate file to keep in sync. The empty-state check is a plain "),
						Code("if"), Text(", and the status pill is a plain "), Code("switch"), Text(".")),
					P(Text("Props are function parameters, so a typo is a compile error, your editor completes them from the signature, "),
						Text("and a rename refactors every call site.")),
				)}

				{Section("translate", "Coming from JSX",
					P(Text("Most of what you know transfers directly. The rest has an obvious Go spelling:")),
					<table>
						<thead><tr><th>JSX</th><th>GSX</th></tr></thead>
						<tbody>
							<tr><td><code>{"<div>…</div>"}</code>, <code>{"<>…</>"}</code>, <code>{"{expr}"}</code></td><td>identical</td></tr>
							<tr><td><code>className</code>, <code>htmlFor</code>, <code>maxLength</code></td><td>identical (or the HTML spelling, your choice)</td></tr>
							<tr><td><code>{"{items.map(i => <li>{i}</li>)}"}</code></td><td>a <code>for</code> loop appending to <code>[]Node</code>, spliced</td></tr>
							<tr><td><code>{"{cond && <p/>}"}</code></td><td><code>{"{If(cond, <p/>)}"}</code>, or plain Go <code>if</code></td></tr>
							<tr><td><code>{"{...props}"}</code></td><td><code>{"{...attrs}"}</code>, where <code>attrs</code> is a <code>[]Node</code></td></tr>
							<tr><td>a props object</td><td>typed function parameters</td></tr>
							<tr><td><code>children</code></td><td>a trailing <code>...Node</code> parameter</td></tr>
							<tr><td><code>{"<Card/>"}</code> resolved by scope</td><td>uppercase tags are function calls, resolved lexically</td></tr>
							<tr><td>hooks, state, a virtual DOM</td><td>none of it — GSX only renders</td></tr>
						</tbody>
					</table>,
					P(Text("GSX is the JSX half of the deal: the syntax, on the server, ahead of time. "),
						Text("For interactivity, reach for whatever you would otherwise pair with server-rendered HTML.")),
				)}

				{Section("output", "No runtime — the generated Go is the artifact",
					P(Text("GSX runs ahead of time and writes a "), Code(".gsx.go"), Text(" file next to each source file. "),
						Text("You check it in and review it like any other code — as the "), Code("InvoiceTable"),
						Text(" output above shows, it is the same function with the tags spelled out.")),
					P(Text("When something renders wrong, you open that file and read exactly what will run. "),
						Text("No reflection, no template parser, no interpreter, no diffing — at run time it is ordinary "),
						Link("https://pkg.go.dev/maragu.dev/gomponents", "gomponents"), Text(" calls you can step through in a debugger.")),
					Note(P(Text("Nothing GSX-specific ships in your binary. The compiler is a build-time tool, so a production build "),
						Text("cannot depend on it and cannot drift from it — "), Code("gsx -check ./..."), Text(" proves that in CI."))),
				)}

				{Section("why", "What you get",
					<div class="cards">
						<div class="card">
							<h3>JSX syntax, Go semantics</h3>
							<p>Tags, fragments, splices and spread work the way you expect. Everything between them is Go, checked by the Go compiler.</p>
						</div>
						<div class="card">
							<h3>Loops and branches, not directives</h3>
							<p><code>for</code>, <code>if</code>, <code>switch</code> and early returns build markup, because a tag is just an expression.</p>
						</div>
						<div class="card">
							<h3>Type-safe by construction</h3>
							<p>Components are functions and props are parameters, so the compiler checks every call site and your editor completes them.</p>
						</div>
						<div class="card">
							<h3>The output is the artifact</h3>
							<p>Generated files are readable Go you check in and review. Debugging is reading code, not tracing a template engine.</p>
						</div>
						<div class="card">
							<h3>Live reload</h3>
							<p><code>gsx dev</code> regenerates, restarts your server and reloads the browser on every save.</p>
						</div>
						<div class="card">
							<h3>Real editor support</h3>
							<p>A gopls-backed language server gives diagnostics, hover and go-to-definition inside <code>.gsx</code> files.</p>
						</div>
					</div>,
				)}

				{Section("start", "Getting started",
					P(Text("Write a "), Code(".gsx"), Text(" file, then generate:")),
					Shell("gsx ./..."),
					P(Text("In CI, check that source and generated output never drift apart:")),
					Shell("gsx -check ./..."),
					P(Text("While developing, get a rebuild and a browser reload on every save:")),
					Shell("gsx dev"),
					P(Text("Next: "), Link("./language.html", "the language reference"), Text(", or "),
						Link("./composition.html", "how to build and organize components"), Text(".")),
				)}
			</>
		),
	}
}
