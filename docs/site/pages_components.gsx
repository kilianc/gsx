package site

func ComponentsPage() Page {
	return Page{
		Slug:     "components",
		Title:    "Components",
		Subtitle: "Components are Go functions. Props are parameters. There is no component runtime.",
		Body: (
			<>
				{Section("basics", "Uppercase tags call functions",
					P(Text("A component is an ordinary function returning "), Code("Node"), Text(". An uppercase tag calls it.")),
					Split(
						GSX(`
func Card(children ...Node) Node {
  return <div class="card">{children}</div>
}

func Page() Node {
  return <Card class="primary"><p>Hello</p></Card>
}
`),
						Out("generated", `
func Card(children ...Node) Node {
	return html.Div(html.Class("card"), Group(children))
}

func Page() Node {
	return Card(html.Class("primary"), html.P(Text("Hello")))
}
`),
					),
					P(Text("Attributes and children both become "), Code("...Node"), Text(" arguments, the same shape gomponents' own helpers use.")),
					Note(P(Text("The rule is purely lexical: lowercase tags ("),
						Code("<div>"), Text(") are always HTML elements, uppercase tags ("), Code("<Card>"),
						Text(") are always function calls. Nothing is resolved by scope."))),
				)}

				{Section("props", "Typed props",
					P(Text("A component can take typed parameters instead of only children. Attributes map to parameters "), Em("by name"), Text(":")),
					Split(
						GSX(`
func Badge(text string, variant string) Node {
  return <span class={"badge badge-" + variant}>{text}</span>
}

<Badge text="Active" variant="success" />
`),
						Out("generated", `Badge("Active", "success")`),
					),
					P(Text("Any Go type works — "), Code("time.Time"), Text(", "), Code("sql.NullString"),
						Text(", your own structs, pointers, slices:")),
					Split(
						GSX(`
func CellTime(t time.Time) Node {
  return <td>{t.Format(time.RFC3339)}</td>
}

<CellTime t={row.CreatedAt} />
`),
						Out("generated", `CellTime(row.CreatedAt)`),
					),
					<h3>Mixing props and children</h3>,
					P(Text("A trailing "), Code("...Node"), Text(" parameter receives the children; named parameters come from attributes:")),
					Split(
						GSX(`
func Section(heading string, children ...Node) Node {
  return <section><h2>{heading}</h2>{children}</section>
}

<Section heading="Tasks">
  <p>content</p>
</Section>
`),
						Out("generated", `Section("Tasks", html.P(Text("content")))`),
					),
					P(Text("Because props are function parameters, a typo is a Go compile error and your editor completes them from the signature.")),
				)}

				{Section("packages", "Components from other packages",
					P(Text("A dotted tag calls a qualified function, including a nested one:")),
					Split(
						GSX(`
import "myapp/ui"

func Page() Node {
  return <ui.Card><p>Hello</p></ui.Card>
}
`),
						Out("generated", `ui.Card(html.P(Text("Hello")))`),
					),
					P(Text("GSX reads the signatures of functions in your module, so typed props work across packages too.")),
				)}

				{Section("patterns", "Patterns",
					<h3>Lists</h3>,
					P(Text("Build a "), Code("[]Node"), Text(" with an ordinary loop and splice it:")),
					GSX(`
var rows []Node
for _, u := range users {
  rows = append(rows, <tr><td>{u.Name}</td></tr>)
}

return <table><tbody>{rows}</tbody></table>
`),
					<h3>Conditionals</h3>,
					P(Text("Use "), Code("If"), Text(" from gomponents for an inline branch, or plain Go for anything larger:")),
					GSX(`
return (
  <div>
    {If(admin, <span class="pill">admin</span>)}
    {If(len(items) == 0, <p class="empty">Nothing here yet.</p>)}
  </div>
)
`),
					P(Text("Go's own control flow works too, since a tag is just an expression:")),
					GSX(`
func Status(s State) Node {
  switch s {
  case Loading:
    return <p>Loading…</p>
  case Failed:
    return <p class="error">Something went wrong.</p>
  }
  return <p>Ready</p>
}
`),
					<h3>Raw HTML</h3>,
					P(Text("Text is escaped. To emit pre-rendered markup, use gomponents' "), Code("Raw"), Text(":")),
					GSX(`<article>{Raw(renderedMarkdown)}</article>`),
					Note(P(Strong("Raw does not escape. "),
						Text("Only pass it markup you generated or have sanitised — never untrusted input."))),
				)}
			</>
		),
	}
}
