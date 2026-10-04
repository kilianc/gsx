package e2e

func init() {
	GSXFunctions["fragment_only"] = func() Node {
		return FragmentOnly(Text("a"), Text("b"))
	}
}

// FragmentOnly is the only markup in this file, and it is a fragment: nothing
// here calls into gomponents/html, so the generated file must not import it.
func FragmentOnly(a, b Node) Node {
	return <>{a}{b}</>
}
