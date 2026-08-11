package highlight

import (
	"strings"
	"testing"
)

// Every byte of the input must appear in exactly one token, or the rendered
// snippet silently loses or duplicates source.
func TestTokensAreLossless(t *testing.T) {
	inputs := []string{
		`func F() Node { return <div class="c">{x}</div> }`,
		"// comment\nvar s = `raw\nstring`\n",
		`<>{/* frag */}<p>hi</p></>`,
		`a < b && c > d`,
		`x := 'q'; y := "\"esc\""`,
		"",
		"<",
		"<div",
		`{`,
		"<p>unclosed",
	}
	for _, in := range inputs {
		var sb strings.Builder
		for _, tok := range Tokens(in) {
			sb.WriteString(tok.Text)
		}
		if sb.String() != in {
			t.Errorf("round trip failed\n in: %q\nout: %q", in, sb.String())
		}
	}
}

func TestClassification(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want map[Class]string // class → a substring that must carry it
	}{
		{
			name: "go keywords and types",
			src:  `func F(s string) Node { return nil }`,
			want: map[Class]string{ClassKeyword: "func", ClassType: "string"},
		},
		{
			name: "comment",
			src:  "// note\nx",
			want: map[Class]string{ClassComment: "// note"},
		},
		{
			name: "raw string",
			src:  "var s = `a\nb`",
			want: map[Class]string{ClassString: "`a\nb`"},
		},
		{
			name: "tag and attribute",
			src:  `<div class="c">hi</div>`,
			want: map[Class]string{ClassTag: "<div", ClassAttr: "class", ClassString: `"c"`},
		},
		{
			name: "closing tag includes its bracket",
			src:  `<p>hi</p>`,
			want: map[Class]string{ClassTag: "</p>"},
		},
		{
			name: "splice braces",
			src:  `<div>{name}</div>`,
			want: map[Class]string{ClassBrace: "{"},
		},
		{
			name: "fragment",
			src:  `<><p>a</p></>`,
			want: map[Class]string{ClassTag: "<>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			byClass := map[Class][]string{}
			for _, tok := range Tokens(tt.src) {
				byClass[tok.Class] = append(byClass[tok.Class], tok.Text)
			}
			for class, want := range tt.want {
				if !containsAny(byClass[class], want) {
					t.Errorf("class %q = %q, want one containing %q", class, byClass[class], want)
				}
			}
		})
	}
}

// A tag inside a splice must highlight as markup, and Go inside that tag must
// highlight as Go — that boundary is the whole reason this exists.
func TestNestedTagInsideSplice(t *testing.T) {
	toks := Tokens(`<div>{If(ok, <p>{name}</p>)}</div>`)

	var tags, braces int
	for _, tok := range toks {
		switch tok.Class {
		case ClassTag:
			tags++
		case ClassBrace:
			braces++
		}
	}
	if tags < 4 {
		t.Errorf("got %d tag tokens, want at least 4 (outer open/close, inner open/close)", tags)
	}
	if braces < 4 {
		t.Errorf("got %d brace tokens, want at least 4 (two splices)", braces)
	}
}

// A `<` that is not a tag must stay unclassified, or ordinary Go comparisons
// render as broken markup.
func TestComparisonIsNotATag(t *testing.T) {
	for _, src := range []string{`a < b`, `a <- ch`, `a << 2`} {
		for _, tok := range Tokens(src) {
			if tok.Class == ClassTag {
				t.Errorf("%q: %q classified as a tag", src, tok.Text)
			}
		}
	}
}

func TestHTMLEscapes(t *testing.T) {
	got := HTML(`<div class="c">a & b</div>`)
	for _, unwanted := range []string{"<div", "a & b"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("output contains unescaped %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, "&amp;") || !strings.Contains(got, "&lt;") {
		t.Errorf("expected escaped entities:\n%s", got)
	}
	if !strings.Contains(got, `class="hl-tg"`) {
		t.Errorf("expected a tag span:\n%s", got)
	}
}

func containsAny(hay []string, needle string) bool {
	for _, h := range hay {
		if strings.Contains(h, needle) {
			return true
		}
	}
	return false
}
