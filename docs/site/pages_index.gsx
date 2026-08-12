package site

func IndexPage() Page {
	return Page{
		Slug:     "index",
		Title:    "GSX",
		Subtitle: "Write HTML inline in ordinary Go functions. No template language, no runtime — just Go you can read.",
		Body:     (
			<>
				<div class="hero-cta">
					<a class="btn btn-primary" href="./language.html">Read the language guide</a>
					<a class="btn" href="./composition.html">Build components</a>
					<a class="btn" href="https://github.com/kilianc/gsx">GitHub</a>
				</div>

				{Shell("go install github.com/kilianc/gsx/cmd/gsx@latest")}

				{Section("simple", "Start simple",
					P(Text("A "), Code(".gsx"), Text(" file is a normal Go file. The only new thing is that a tag is an expression.")),
					Split(
						GSX(`
package ui

func Hello() Node {
  return (
    <main class="page">
      <h1>Hello</h1>
      <p>Welcome to GSX.</p>
    </main>
  )
}
`),
						Out("rendered html", `
<main class="page">
  <h1>Hello</h1>
  <p>Welcome to GSX.</p>
</main>
`),
					),
				)}

				{Section("power", "Then mix in Go",
					P(Text("Because markup is an expression, everything you already do in Go still works. Loops build lists. "),
						Text("Conditionals pick branches. Values come from variables and function calls. ")),
					P(Text("There is no template language between you and the page — no "), Code("{{range}}"),
						Text(", no partials to register, no separate file to keep in sync.")),
					GSX(`
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
					P(Text("Props are function parameters, so a typo is a compile error and your editor completes them from the signature.")),
				)}

				{Section("output", "And you can read the output",
					P(Text("GSX runs ahead of time and writes a "), Code(".gsx.go"), Text(" file next to each source file. "),
						Text("You check it in and review it like any other code.")),
					Out("profile.gsx.go", `
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
`),
					P(Text("That is the whole trick. When something renders wrong, you open the generated file and read exactly what will run — no reflection, no template parser, no interpreter at run time.")),
				)}

				{Section("why", "What you get",
					<div class="cards">
						<div class="card">
							<h3>The output is the artifact</h3>
							<p>Generated files are readable Go you check in and review. Debugging is reading code, not tracing a template engine.</p>
						</div>
						<div class="card">
							<h3>No runtime</h3>
							<p>Everything happens at generation time. At run time it is ordinary <a href="https://pkg.go.dev/maragu.dev/gomponents">gomponents</a>.</p>
						</div>
						<div class="card">
							<h3>Type-safe by construction</h3>
							<p>Components are functions and props are parameters, so the Go compiler checks every call site.</p>
						</div>
						<div class="card">
							<h3>Familiar syntax</h3>
							<p>Fragments, <code>{"{expr}"}</code> splices, <code>className</code>, spread attributes, <code>{"{/* comments */}"}</code> — JSX habits carry over.</p>
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
