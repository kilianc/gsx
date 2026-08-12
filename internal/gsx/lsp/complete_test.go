package lsp

import (
	"strings"
	"testing"
)

// at builds a source and cursor offset from a string marking the caret with |.
func at(t *testing.T, marked string) (string, int) {
	t.Helper()
	i := strings.Index(marked, "|")
	if i < 0 {
		t.Fatalf("fixture has no | caret: %q", marked)
	}
	return marked[:i] + marked[i+1:], i
}

func TestCompletionContext(t *testing.T) {
	tests := []struct {
		name   string
		marked string
		want   completionKind
		prefix string
	}{
		{"after open bracket", "func F() Node { return <| }", completeTag, ""},
		{"partial tag name", "func F() Node { return <di| }", completeTag, "di"},
		{"partial component", "func F() Node { return <Car| }", completeTag, "Car"},
		{"dotted component", "func F() Node { return <ui.Ca| }", completeTag, "ui.Ca"},

		{"inside start tag", `func F() Node { return <div | }`, completeAttribute, ""},
		{"partial attribute", `func F() Node { return <div cla| }`, completeAttribute, "cla"},
		{"second attribute", `func F() Node { return <div id="x" cl| }`, completeAttribute, "cl"},

		{"closing tag", "func F() Node { return <div>hi</| }", completeCloseTag, ""},

		// Inside a quoted value the user is writing a value, not an attribute.
		{"inside attribute value", `func F() Node { return <div class="ca| }`, completeNone, ""},

		// Inside a splice the language is Go again.
		{"inside splice", "func F() Node { return <div>{na| }", completeNone, ""},

		// Plain Go must be left to gopls.
		{"plain go", "func F() Node { x := stri| }", completeNone, ""},
		{"after finished tag", "func F() Node { return <div>hi| }", completeNone, ""},
		{"comparison", "func F() bool { return a < b| }", completeNone, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, off := at(t, tt.marked)
			got := completionAt(src, off)
			if got.kind != tt.want {
				t.Errorf("kind = %v, want %v (prefix %q)", got.kind, tt.want, got.prefix)
			}
			if got.kind != completeNone && got.prefix != tt.prefix {
				t.Errorf("prefix = %q, want %q", got.prefix, tt.prefix)
			}
		})
	}
}

func TestCloseTagCompletionNamesTheOpenTag(t *testing.T) {
	tests := []struct {
		marked string
		want   string
	}{
		{"return <div>hi</|", "div"},
		{"return <section><p>hi</p></|", "section"},
		{"return <div><span>a</span>b</|", "div"},
		// A self-closing tag is never open.
		{"return <div><br />x</|", "div"},
	}
	for _, tt := range tests {
		src, off := at(t, tt.marked)
		ctx := completionAt(src, off)
		if ctx.kind != completeCloseTag {
			t.Errorf("%q: kind = %v", tt.marked, ctx.kind)
			continue
		}
		if ctx.tag != tt.want {
			t.Errorf("%q: tag = %q, want %q", tt.marked, ctx.tag, tt.want)
		}
	}
}

func TestTagCompletionsIncludeElementsAndLocalComponents(t *testing.T) {
	src := `package ui

func Card(children ...Node) Node { return <div>{children}</div> }
func Badge(text string) Node     { return <span>{text}</span> }
func helper() string             { return "" }
func NotAComponent() string      { return "" }

func Page() Node { return <
`
	items := completionsFor(completionContext{kind: completeTag}, src)

	labels := map[string]completionItem{}
	for _, it := range items {
		labels[it.Label] = it
	}

	for _, want := range []string{"Card", "Badge"} {
		it, ok := labels[want]
		if !ok {
			t.Errorf("missing component %q", want)
			continue
		}
		if it.Detail != "component" {
			t.Errorf("%q detail = %q", want, it.Detail)
		}
		// Components must sort above the HTML elements.
		if !strings.HasPrefix(it.SortText, "0") {
			t.Errorf("%q sortText = %q, want it to sort first", want, it.SortText)
		}
	}

	// Functions that do not return Node are not components.
	for _, unwanted := range []string{"helper", "NotAComponent"} {
		if _, ok := labels[unwanted]; ok {
			t.Errorf("%q should not be offered as a component", unwanted)
		}
	}

	for _, want := range []string{"div", "section", "table", "blockquote"} {
		if _, ok := labels[want]; !ok {
			t.Errorf("missing HTML element %q", want)
		}
	}
}

func TestAttributeCompletions(t *testing.T) {
	items := attributeCompletions()

	byLabel := map[string]completionItem{}
	for _, it := range items {
		byLabel[it.Label] = it
	}

	// A value-bearing attribute lands the caret between the quotes.
	if got := byLabel["class"].InsertText; got != `class="$1"` {
		t.Errorf("class insertText = %q", got)
	}
	// A boolean attribute takes no value.
	if got := byLabel["disabled"].InsertText; got != "" {
		t.Errorf("disabled insertText = %q, want none", got)
	}
	if got := byLabel["disabled"].Detail; got != "boolean attribute" {
		t.Errorf("disabled detail = %q", got)
	}
	// JSX spellings are offered too.
	for _, want := range []string{"className", "htmlFor", "maxLength"} {
		if _, ok := byLabel[want]; !ok {
			t.Errorf("missing JSX spelling %q", want)
		}
	}
}

func TestOffsetOf(t *testing.T) {
	src := "ab\ncde\nf"
	tests := []struct {
		line, char, want int
	}{
		{0, 0, 0},
		{0, 2, 2},
		{1, 0, 3},
		{1, 3, 6},
		{2, 1, 8},
		{99, 0, len(src)},
	}
	for _, tt := range tests {
		if got := offsetOf(src, tt.line, tt.char); got != tt.want {
			t.Errorf("offsetOf(%d,%d) = %d, want %d", tt.line, tt.char, got, tt.want)
		}
	}
}
