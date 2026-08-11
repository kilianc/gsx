package site

func OrganizingPage() Page {
	return Page{
		Slug:     "organizing",
		Title:    "Organizing your codebase",
		Subtitle: "Where components live, how packages divide up, and what changes once markup is Go.",
		Body: (
			<>
				{Section("files", "Files and generated output",
					P(Text("Each "), Code("foo.gsx"), Text(" produces a "), Code("foo.gsx.go"), Text(" beside it. "),
						Text("Both are checked in. The generated file is reviewed like any other code — that is the point of generating readable output.")),
					<ul>
						<li>Edit <code>.gsx</code>. Never edit <code>.gsx.go</code>; the next generation overwrites it.</li>
						<li>Commit both, so a fresh clone builds without running the generator.</li>
						<li>Run <code>gsx -check ./...</code> in CI so the two cannot drift apart.</li>
					</ul>,
					P(Text("A "), Code(".gsx"), Text(" file is a normal Go file, so a package can mix them freely with "),
						Code(".go"), Text(" files. Put plain helpers, types and tests in "), Code(".go"),
						Text("; put anything containing markup in "), Code(".gsx"), Text(".")),
					Note(P(Text("Mark generated files in "), Code(".gitattributes"), Text(" so reviews collapse them by default:")),
						Out(".gitattributes", "*.gsx.go linguist-generated=true -diff")),
				)}

				{Section("layouts", "Three layouts that work",
					P(Text("Pick based on how your team actually navigates the code, not on which looks tidiest in a diagram.")),

					<h3>1. One ui package</h3>,
					P(Text("Best up to roughly a few dozen components. Everything is one import and one place to look.")),
					Out("project", `
myapp/
  main.go
  ui/
    shell.gsx        Shell, Nav, Footer
    button.gsx       Button, IconButton
    card.gsx         Card, Panel
    table.gsx        Table, Row, Cell
    ui.go            shared types: Variant, Size
  pages/
    home.gsx         Home
    settings.gsx     Settings
`),
					P(Text("Callers write "), Code("<ui.Card>"), Text(" and "), Code("<ui.Button variant={ui.Primary}>"),
						Text(". The qualifier makes it obvious at a glance which tags are yours.")),

					<h3>2. Feature packages</h3>,
					P(Text("Once features have real surface area, colocate their markup with their logic and keep only the genuinely shared pieces in "),
						Code("ui"), Text(".")),
					Out("project", `
myapp/
  ui/                shared design system only
    shell.gsx
    button.gsx
  billing/
    handler.go
    invoice.go       domain types
    invoice.gsx      InvoiceTable, InvoiceRow
  teams/
    handler.go
    members.gsx      MemberList, InviteForm
`),
					P(Text("The rule of thumb: a component moves into "), Code("ui"), Text(" when a "),
						Em("second"), Text(" feature needs it — not before.")),

					<h3>3. Colocated pages</h3>,
					P(Text("For content-heavy sites where each page is mostly unique, a flat page directory beats a component hierarchy:")),
					Out("project", `
myapp/
  site/
    layout.gsx       Layout, Header, Footer
    style.go         the stylesheet
    page_index.gsx   IndexPage
    page_about.gsx   AboutPage
    site.go          page registry, rendering
`),
					Note(P(Text("This is how "), Strong("this site"), Text(" is built — see "),
						Link("https://github.com/kilianc/gsx/tree/main/docs", "docs/"), Text(" in the repository."))),
				)}

				{Section("naming", "Naming and visibility",
					<ul>
						<li><strong>Exported components are the API.</strong> Unexported ones are implementation detail — use them freely for pieces only one file needs.</li>
						<li><strong>Name for the thing, not the markup.</strong> <code>EmptyState</code> and <code>InvoiceRow</code>, not <code>DivWrapper</code> or <code>GreyBox</code>.</li>
						<li><strong>Keep the package qualifier in mind.</strong> <code>ui.Button</code> reads well; <code>ui.UIButton</code> does not.</li>
					</ul>,
					P(Text("Because tags resolve lexically, a component whose name collides with an HTML element is fine: "),
						Code("<Header>"), Text(" is your function, "), Code("<header>"), Text(" is the element.")),
					Compiled(`
func Header(title string, children ...Node) Node {
  return <header><h1>{title}</h1>{children}</header>
}

func Page() Node {
  return (
    <div>
      <Header title="Hello"><p>content</p></Header>
      <header><p>a plain element</p></header>
    </div>
  )
}
`),
				)}

				{Section("data", "Keep data out of components",
					P(Text("A component should take what it renders, not fetch it. That keeps it testable, reusable and obvious at the call site.")),
					Split(
						GSX(`
{/* Avoid: hidden dependency, untestable */}
func InvoiceTable() Node {
  rows := db.Query("SELECT ...")
  return <table>{...}</table>
}
`),
						GSX(`
{/* Prefer: data in, markup out */}
func InvoiceTable(invoices []Invoice) Node {
  return <table>{...}</table>
}
`),
					),
					P(Text("Handlers fetch, components render:")),
					GSX(`
func (s *Server) invoices(w http.ResponseWriter, r *http.Request) {
  invoices, err := s.store.Invoices(r.Context())
  if err != nil {
    http.Error(w, "…", http.StatusInternalServerError)
    return
  }

  w.Header().Set("Content-Type", "text/html; charset=utf-8")
  _ = ui.Shell("Invoices", billing.InvoiceTable(invoices)).Render(w)
}
`),
				)}

				{Section("imports", "Imports and dot-imports",
					P(Text("Generated files dot-import gomponents so "), Code("Node"), Text(", "), Code("Text"),
						Text(", "), Code("If"), Text(" and "), Code("Group"), Text(" are available unqualified, and import "),
						Code("gomponents/html"), Text(" under the "), Code("html"), Text(" prefix.")),
					P(Text("Inside a tag or a "), Code("{...}"), Text(" splice you write bare names — "), Code("Class"),
						Text(", "), Code("If"), Text(" — and GSX qualifies them. In "), Em("plain"), Text(" Go code in the same file, "),
						Text("write the prefix yourself:")),
					GSX(`
{/* inside markup: bare names are resolved */}
<div class="x" {If(active, Class("on"))}>…</div>

{/* in plain Go: qualify explicitly */}
func attrs() []Node {
  return []Node{html.Class("x"), Attr("data-k", "v")}
}
`),
					P(Text("Your own declarations always win. A component or variable named "), Code("Section"),
						Text(" shadows "), Code("html.Section"), Text(" exactly as it would in ordinary Go.")),
				)}
			</>
		),
	}
}
