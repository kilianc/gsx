package helpers

import g "maragu.dev/gomponents"

func MakeNode(s string) g.Node {
	return g.Text(s)
}

func MakeString(s string) string {
	return s
}
