package helpers

import (
	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

func MakeNode(s string) g.Node {
	return g.Text(s)
}

func MakeString(s string) string {
	return s
}

func Wrapper(children ...g.Node) g.Node {
	return html.Div(append([]g.Node{html.Class("wrapper")}, children...)...)
}
