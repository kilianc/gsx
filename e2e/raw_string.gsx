package e2e

func init() {
	GSXFunctions["raw_string"] = func() Node {
		return RawString()
	}
}

// A raw string literal spliced into markup must survive byte for byte. The
// pretty-printer collapses generated Go onto one line and re-indents it to the
// call site; doing either inside a literal silently rewrites the program's data.
func RawString() Node {
	return (
		<div>
			<style>{`
.card {
  color: red;
}
`}</style>
			<pre>{`line one
	line two, tab indented
line three`}</pre>
		</div>
	)
}
