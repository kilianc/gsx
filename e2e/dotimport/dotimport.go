package dotimport

import (
	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Section intentionally collides with html.Section to test qualified imports.
func Section(heading string, children ...g.Node) g.Node {
	return html.Section(
		append([]g.Node{html.H2(g.Text(heading))}, children...)...,
	)
}

func EmptyState(message string) g.Node {
	return html.Div(html.Class("empty"), html.P(g.Text(message)))
}

func Page(children ...g.Node) g.Node {
	return html.Main(children...)
}
