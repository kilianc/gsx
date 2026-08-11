package site

func LanguagePage() Page {
	return Page{
		Slug:     "language",
		Title:    "Language reference",
		Subtitle: "Everything GSX adds to Go, and exactly what each form compiles to.",
		Body: (
			<>
				{Section("tags", "Tag expressions",
					P(Text("A tag is an expression. It can go anywhere a Go expression can — returned, assigned, appended to a slice, passed as an argument.")),
					GSX(`
banner := <div class="banner">Welcome</div>
items = append(items, <li>{name}</li>)
return <main>{banner}</main>
`),
					P(Text("Lowercase tags are HTML elements. Uppercase tags call Go functions. Self-closing tags use "), Code("/>"), Text(", as in JSX — "), Code("<br>"), Text(" alone is not valid.")),
					Note(P(Text("A multi-line tag needs parentheses so Go does not end the statement at the newline:")),
						GSX(`
return (
  <div>
    <p>Hello</p>
  </div>
)
`)),
				)}

				{Section("splices", "Splices",
					P(Text("A "), Code("{expr}"), Text(" splice embeds a Go expression. In child position the expression must be a "),
						Code("string"), Text(", a "), Code("Node"), Text(", or a "), Code("[]Node"), Text(".")),
					Split(
						GSX(`
<div>
  {name}          {/* string */}
  {banner}        {/* Node   */}
  {items}         {/* []Node */}
</div>
`),
						Out("generated", `
html.Div(
	Text(name),
	banner,
	Group(items),
)
`),
					),
					P(Text("GSX picks the wrapper from the type it infers. There is no implicit stringification: splicing something that is neither a string nor a Node is a Go compile error at the call site, not a silent "), Code("%v"), Text(".")),
				)}

				{Section("fragments", "Fragments",
					P(Text("Two sibling tags cannot sit adjacent in one Go expression. Wrap them in a fragment:")),
					Split(
						GSX(`
return (
  <>
    <h1>Title</h1>
    <p>Subtitle</p>
  </>
)
`),
						Out("generated", `
Group{
	html.H1(Text("Title")),
	html.P(Text("Subtitle")),
}
`),
					),
					P(Text("A fragment renders its children with no wrapping element.")),
				)}

				{Section("text", "Text and whitespace",
					P(Text("Literal text follows JSX's whitespace rules, so markup renders the way it reads.")),
					Split(
						GSX(`
<p>
  This sentence is written
  across three source lines
  but renders as one.
</p>
`),
						Out("rendered", `
<p>This sentence is written across three source lines but renders as one.</p>
`),
					),
					P(Text("A run of text containing a line break has each line trimmed, blank lines dropped, and the rest joined with a single space. A run "),
						Em("without"), Text(" a line break is kept byte for byte, so inline spacing survives:")),
					GSX(`<p><b>bold</b> then <i>italic</i></p>`),
					<h3>Entities</h3>,
					P(Text("HTML entities are decoded at compile time and escaped again on render, so what you write is what the reader sees.")),
					Split(
						GSX(`<p>Tom &amp; Jerry &lt;3 caf&eacute;</p>`),
						Out("rendered", `<p>Tom &amp; Jerry &lt;3 café</p>`),
					),
					P(Text("Only well-formed references are decoded — a name or numeric form ending in "), Code(";"),
						Text(". Unlike a browser, GSX leaves a reference without its semicolon alone:")),
					<table>
						<thead><tr><th>You write</th><th>GSX</th><th>A browser</th></tr></thead>
						<tbody>
							<tr><td><code>&amp;amp;</code></td><td><code>&amp;</code></td><td><code>&amp;</code></td></tr>
							<tr><td><code>&amp;#65;</code></td><td><code>A</code></td><td><code>A</code></td></tr>
							<tr><td><code>?page=1&amp;next=2</code></td><td>unchanged</td><td><code>?page=1¬ext=2</code></td></tr>
							<tr><td><code>AT&amp;T</code></td><td>unchanged</td><td>unchanged</td></tr>
						</tbody>
					</table>,
				)}

				{Section("comments", "Comments",
					P(Text("Ordinary Go comments work anywhere Go code does. Inside markup, use a JSX comment — it is dropped entirely:")),
					GSX(`
<div>
  {/* Not emitted, in child or attribute position. */}
  <p>Hello</p>
</div>
`),
				)}

				{Section("attributes", "Attributes",
					P(Text("Attribute values are either a quoted literal or a "), Code("{expr}"), Text(" splice.")),
					Split(
						GSX(`
<a href="/about" class={cls} data-id={id} download>
  About
</a>
`),
						Out("generated", `
html.A(
	html.Href("/about"),
	html.Class(cls),
	Attr("data-id", id),
	Attr("download"),
	Text("About"),
)
`),
					),
					<h3>Boolean attributes</h3>,
					P(Text("A valueless attribute is always present. Give it a "), Code("bool"), Text(" splice to make it conditional, as in JSX:")),
					Split(
						GSX(`<input required disabled={locked} />`),
						Out("generated", `
html.Input(
	html.Required(),
	If(locked, html.Disabled()),
)
`),
					),
					<h3>JSX spellings</h3>,
					P(Text("Both the HTML and JSX names are accepted, matched case-insensitively, so pasted JSX compiles:")),
					GSX(`
<label htmlFor="email">Email</label>
<div className="card">…</div>
<input autoComplete="off" maxLength="120" />
`),
					P(Code("data-*"), Text(" and "), Code("aria-*"), Text(" pass through as written.")),
					<h3>Spread</h3>,
					P(Text("A "), Code("[]Node"), Text(" of attributes can be applied to an element:")),
					Split(
						GSX(`
func shared() []Node {
  return []Node{Attr("class", "x"), Attr("data-k", "v")}
}

<span {...shared()}>one</span>
`),
						Out("generated", `html.Span(Group(shared()), Text("one"))`),
					),
					<h3>Attribute nodes</h3>,
					P(Text("A bare splice in attribute position injects an attribute node, which is how conditional attributes are written:")),
					GSX(`<div class="btn" {If(active, Class("is-active"))}>ok</div>`),
					Note(P(Strong("Known limitation. "),
						Text("A literal "), Code("class"), Text(" plus a spliced "), Code("Class(...)"),
						Text(" emits two separate "), Code("class"), Text(" attributes — gomponents does not merge them. Build the string in Go, or use "),
						Code("components.Classes"), Text("."))),
				)}

				{Section("errors", "Errors",
					P(Text("Parse errors carry a position and a source snippet:")),
					Out("terminal", `
page.gsx:4:32: mismatched closing tag </span>, expected </div>

  4 |  return <div class="card">hello</span>
    |                                ^
`),
					P(Text("The same message appears as an editor diagnostic on the offending character, and as a full-page overlay under "),
						Link("./live-reload.html", "gsx dev"), Text(".")),
				)}
			</>
		),
	}
}
