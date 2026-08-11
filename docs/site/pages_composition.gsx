package site

func CompositionPage() Page {
	return Page{
		Slug:     "composition",
		Title:    "Reusable components",
		Subtitle: "How to factor markup into components that compose — and how to organize them once you have more than a handful.",
		Body: (
			<>
				{Section("mental-model", "The mental model",
					P(Text("A component is a function that returns a "), Code("Node"), Text(". That is the entire abstraction.")),
					P(Text("Everything you know about factoring Go functions applies unchanged: extract when there is repetition or a name worth having, "),
						Text("pass what varies as parameters, return early, keep the signature honest. There is no component lifecycle, no registration step, no base class.")),
					Note(P(Text("If you find yourself asking \"can a component do X?\", the answer is whatever a Go function can do."))),
				)}

				{Section("when", "When to extract one",
					P(Text("Resist extracting on line count alone. A 60-line page that reads top to bottom is easier to follow than six components that each hide four lines.")),
					P(Strong("Good reasons to extract:")),
					<ul>
						<li>The markup is <strong>used in more than one place</strong>, and should change in both.</li>
						<li>It has a <strong>name your team already says out loud</strong> — "the empty state", "a stat tile", "the page shell".</li>
						<li>It <strong>encapsulates a rule</strong>, like which variants a badge allows, so callers cannot get it wrong.</li>
						<li>It lets you <strong>test the rule</strong> without rendering the whole page.</li>
					</ul>,
					P(Strong("Weak reasons:")),
					<ul>
						<li>The function feels long. Length alone is not complexity.</li>
						<li>It might be reused one day. Extract on the second use, not the first guess.</li>
					</ul>,
				)}

				{Section("children", "Children: the slot pattern",
					P(Text("A trailing "), Code("...Node"), Text(" parameter accepts children, which is how you write a wrapper that does not care what goes inside it.")),
					Compiled(`
func Card(children ...Node) Node {
  return (
    <section class="card">
      {children}
    </section>
  )
}

func Revenue() Node {
  return (
    <Card>
      <h2>Revenue</h2>
      <p>Up 12%</p>
    </Card>
  )
}
`),
					P(Text("This is the single most useful pattern in the language. Layouts, cards, modals, panels and list wrappers are all the same shape.")),
				)}

				{Section("named-slots", "Named slots",
					P(Text("When a component has more than one hole, give it "), Code("Node"), Text("-typed parameters. "),
						Text("Each one is a slot the caller fills, and the compiler makes sure they do.")),
					GSX(`
func Panel(title Node, actions Node, children ...Node) Node {
  return (
    <section class="panel">
      <header class="panel-head">
        <div class="panel-title">{title}</div>
        <div class="panel-actions">{actions}</div>
      </header>
      <div class="panel-body">{children}</div>
    </section>
  )
}
`),
					P(Text("Call it with tags for the named slots and ordinary children for the body:")),
					CompiledBelow(`
func Panel(title Node, actions Node, children ...Node) Node {
  return <section class="panel">{title}{actions}{children}</section>
}

func Team() Node {
  return (
    <Panel
      title={<h2>Team</h2>}
      actions={<a class="btn" href="/team/new">Invite</a>}
    >
      <p>Four people.</p>
    </Panel>
  )
}
`),
					Note(P(Text("A slot that is often empty reads better as a "), Code("...Node"), Text(" child than as a "),
						Code("Node"), Text(" parameter you keep passing "), Code("nil"), Text(" to."))),
				)}

				{Section("props", "Props over stringly-typed markup",
					P(Text("Because props are ordinary parameters, use real types and let the compiler enforce the rules.")),
					Split(
						GSX(`
{/* Weak: any string goes, typos render silently */}
func Badge(variant string, children ...Node) Node {
  return <span class={"badge badge-" + variant}>{children}</span>
}

<Badge variant="sucess">Active</Badge>
`),
						GSX(`
{/* Strong: only defined variants compile */}
type Variant string

const (
  Success Variant = "success"
  Warning Variant = "warning"
  Danger  Variant = "danger"
)

func Badge(variant Variant, children ...Node) Node {
  return <span class={"badge badge-" + string(variant)}>{children}</span>
}

<Badge variant={Success}>Active</Badge>
`),
					),
					P(Text("The second version costs four lines and removes an entire category of bug. This is the main thing GSX buys you over a template language.")),
				)}

				{Section("composing", "Composing components",
					P(Text("Components take and return "), Code("Node"), Text(", so they nest without ceremony:")),
					GSX(`
func Dashboard(u User, stats []Stat) Node {
  return (
    <Shell title="Dashboard" user={u}>
      <Grid>
        {statTiles(stats)}
      </Grid>
      <Panel title={<h2>Recent</h2>}>
        <ActivityList items={u.Recent} />
      </Panel>
    </Shell>
  )
}

func statTiles(stats []Stat) []Node {
  var out []Node
  for _, s := range stats {
    out = append(out, <StatTile label={s.Label} value={s.Value} />)
  }
  return out
}
`),
					P(Text("Note "), Code("statTiles"), Text(" returns "), Code("[]Node"), Text(" and is spliced directly. "),
						Text("A helper that builds a list does not need to be a component — it is just a function.")),
				)}

				{Section("layouts", "Layouts",
					P(Text("A layout is a component whose children are the page. Take the parts that vary as props:")),
					CompiledBelow(`
func Shell(title string, children ...Node) Node {
  return (
    <>
      {Raw("<!doctype html>")}
      <html lang="en">
        <head>
          <meta charset="utf-8" />
          <title>{title + " · Acme"}</title>
          <link rel="stylesheet" href="/static/app.css" />
        </head>
        <body>
          <main>{children}</main>
        </body>
      </html>
    </>
  )
}
`),
					P(Text("Every page becomes a call to "), Code("Shell"), Text(", so the shared chrome lives in exactly one place.")),
				)}

				{Section("testing", "Testing components",
					P(Text("A component is a function returning something with a "), Code("Render"), Text(" method, so testing it needs no framework:")),
					GSX(`
func TestBadgeVariant(t *testing.T) {
  var sb strings.Builder
  if err := Badge(Success, Text("Active")).Render(&sb); err != nil {
    t.Fatal(err)
  }
  if !strings.Contains(sb.String(), "badge-success") {
    t.Errorf("got %s", sb.String())
  }
}
`),
					P(Text("Assert on the rendered string for small components. For pages, a golden file is usually clearer than a pile of "),
						Code("Contains"), Text(" checks — write the expected HTML to a file and compare.")),
				)}
			</>
		),
	}
}
