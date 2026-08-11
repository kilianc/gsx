package e2e

import (
	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Summary shadows html.Summary from a different file in the same package.
func Summary(s string) g.Node {
	return html.P(html.Class("summary"), g.Text(s))
}
