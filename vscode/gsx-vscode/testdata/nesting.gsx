package ui

// Same-name nesting must not close the outer element early.
func Nested() Node {
	return <div><div>inner</div>outer</div>
	//= outer -> meta.element.html
	//= !outer -> keyword
}

// Prose between tags is text, not Go. A word that happens to be a Go keyword
// must not be coloured as one.
func Prose() Node {
	return <p>return the package for range go func</p>
	//= !return the package -> keyword
	//= return the package -> meta.element.html
}

// A tag nested inside a splice returns to markup scoping.
func InSplice(ok bool) Node {
	return <div>{If(ok, <p>yes</p>)}</div>
	//= <p -> entity.name.tag
	//= If -> meta.embedded.block.go
}

// A composite literal inside a splice must not end the splice at its brace.
func Composite() Node {
	return <div>{f(map[string]int{"a": 1})}</div>
	//= map[string]int -> meta.embedded.block.go
	//= </div -> entity.name.tag
}

// Self-closing elements leave the surrounding scope correctly.
func SelfClosing() Node {
	return <div><img src="/a.png" /><br />after</div>
	//= after -> meta.element.html
	//= </div -> entity.name.tag
}

// Multi-line tags.
func MultiLine(cls string) Node {
	return (
		<section
			class={cls}
			data-role="main"
		>
			<p>body</p>
			//= <p -> entity.name.tag
			//= body -> meta.element.html
		</section>
	)
}
