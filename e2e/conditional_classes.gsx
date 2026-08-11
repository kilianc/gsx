package e2e

func init() {
	GSXFunctions["conditional_classes"] = func() Node {
		return ConditionalClasses(true)
	}
}

// KNOWN LIMITATION: a literal `class` plus a spliced `Class(...)` node emits two
// separate class attributes, because gomponents does not merge them. See the
// golden. Class merging is tracked as part of the attribute-handling work.
func ConditionalClasses(active bool) Node {
	return <div class="btn extra" {If(active, Class("is-active"))}>ok</div>
}
