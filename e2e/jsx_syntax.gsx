package e2e

func init() {
	GSXFunctions["jsx_syntax"] = func() Node {
		return JSXSyntax("world")
	}
}

// JSXSyntax exercises the JSX-compatible syntax: fragments, comments, text
// whitespace normalization, and HTML entity decoding.
func JSXSyntax(name string) Node {
	return (
		<div>
			{/* Comments are dropped entirely, including this one. */}
			<>
				<h1>Fragments group siblings</h1>
				<p>without emitting a wrapper</p>
			</>

			{/* Indented text reads the way it looks. */}
			<p>
				This sentence is written
				across three source lines
				but renders as one.
			</p>

			{/* Whitespace-only runs between tags carry no content. */}
			<ul>
				<li>one</li>
				<li>two</li>
			</ul>

			{/* A single-line run keeps its spaces, so inline markup stays readable. */}
			<p><b>bold</b> then <i>italic</i></p>

			{/* Entities are decoded at compile time, then escaped on render. */}
			<p>Tom &amp; Jerry &lt;3 &#65;&#x42; caf&eacute;</p>

			<span>{name}</span>
		</div>
	)
}
