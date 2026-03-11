package helpers

import (
	"database/sql"
	"time"

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

func SectionHeading(text string) g.Node {
	return html.H2(g.Text(text))
}

func Badge(text string, variant string) g.Node {
	return html.Span(html.Class("badge badge-"+variant), g.Text(text))
}

func SectionWithHeading(heading string, children ...g.Node) g.Node {
	return html.Section(
		append([]g.Node{html.H2(g.Text(heading))}, children...)...,
	)
}

func StatusDot(active bool) g.Node {
	cls := "dot"
	if active {
		cls += " active"
	}
	return html.Span(html.Class(cls))
}

func EmptyState(message string) g.Node {
	return html.Div(html.Class("empty"), html.P(g.Text(message)))
}

type LinkData struct {
	URL  string
	Text string
}

func CellTime(t time.Time) g.Node {
	return html.Td(g.Text(t.Format(time.RFC3339)))
}

func CellLink(link LinkData) g.Node {
	return html.Td(html.A(html.Href(link.URL), g.Text(link.Text)))
}

func CellNullText(value sql.NullString) g.Node {
	if !value.Valid {
		return html.Td(g.Text("-"))
	}
	return html.Td(g.Text(value.String))
}

func TimestampSection(t time.Time, children ...g.Node) g.Node {
	return html.Section(
		append([]g.Node{html.Span(g.Text(t.Format(time.RFC3339)))}, children...)...,
	)
}
