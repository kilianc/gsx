package e2e

func init() {
	GSXFunctions["conditional_classes"] = func() Node {
		return ConditionalClasses(true)
	}
}

func ConditionalClasses(active bool) Node {
	return <div class="btn extra" {If(active, Class("is-active"))}>ok</div>
}
