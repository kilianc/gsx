package e2e

func init() {
	GSXFunctions["name_shadowing"] = func() Node {
		return NameShadowing()
	}
}

// Article, Code and Summary all collide with gomponents/html exports. A local
// declaration must win, exactly as it would in ordinary Go — otherwise calls to
// them inside a `{...}` splice are rewritten to html.Article, html.Code and
// html.Summary, and the generated file does not compile.
func Article(id string, body Node) Node {
	return <article id={id}>{body}</article>
}

func Code(s string) Node {
	return <code class="inline">{s}</code>
}

// Summary is defined in a sibling file (helpers_shadowing.go) to prove that
// shadowing is resolved across the whole package, not just this file.

func NameShadowing() Node {
	return (
		<div>
			{Article("one", Code("shadowed"))}
			{Summary("from a sibling file")}
			<section id="plain">
				<code>a real element</code>
			</section>
		</div>
	)
}
