package e2e

func init() {
	GSXFunctions["html_coverage"] = func() Node {
		return HTMLCoverage()
	}
}

// sharedAttrs is built once and spread onto several elements.
//
// This is plain Go rather than a tag expression, so gomponents/html helpers
// would need an explicit `html.` prefix here. The dot-imported core Attr is
// enough for the attributes this fixture needs.
func sharedAttrs() []Node {
	return []Node{Attr("class", "shared"), Attr("data-kind", "demo")}
}

// HTMLCoverage exercises elements and attributes that used to fall back to the
// untyped El("tag", ...) and Attr("name", v) forms, plus JSX attribute
// spellings and spread attributes.
func HTMLCoverage() Node {
	return (
		<article>
			{/* Elements with no typed helper before: table, section content, inline markup. */}
			<table>
				<thead>
					<tr><th scope="col">Name</th><th scope="col">Role</th></tr>
				</thead>
				<tbody>
					<tr><td>Ada</td><td><em>engineer</em></td></tr>
				</tbody>
			</table>

			<figure>
				<img src="/a.png" alt="A" width="64" height="64" />
				<figcaption>A caption with <strong>bold</strong> and <code>code</code>.</figcaption>
			</figure>

			{/* JSX spellings resolve to the same attributes as the HTML ones. */}
			<label htmlFor="email">Email</label>
			<input id="email" type="email" autoComplete="off" maxLength="120" required />

			<div className="jsx-spelling">className is class</div>

			{/* Spread applies a prebuilt []Node of attributes. */}
			<span {...sharedAttrs()}>spread one</span>
			<button type="button" {...sharedAttrs()} disabled>spread two</button>

			<details open>
				<summary>More</summary>
				<blockquote cite="https://example.com">Quoted.</blockquote>
			</details>
		</article>
	)
}
