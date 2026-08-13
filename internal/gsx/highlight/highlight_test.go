package highlight

import (
	"slices"
	"strings"
	"testing"

	"github.com/kilianc/gsx/internal/gsx/parse"
)

// Every byte of the input must appear in exactly one token, or the rendered
// snippet silently loses or duplicates source.
func TestTokensAreLossless(t *testing.T) {
	inputs := []string{
		`func F() Node { return <div class="c">{x}</div> }`,
		"// comment\nvar s = `raw\nstring`\n",
		`<>{/* frag */}<p>hi</p></>`,
		`a < b && c > d`,
		`a<b`,
		`a<<b`,
		`<p class={m[a<b]}>hi</p>`,
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

// The highlighter carries its own lexer, so its rule for telling a tag from a
// comparison can drift from the parser's — and silently, since nothing renders
// the two side by side. This pins both halves: the tokens emitted, and
// agreement with parse on whether there is a tag there at all.
func TestGoCodeTagDetection(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string // every token classified as a tag, in order
	}{
		// A `<` after an operand is a comparison or a shift.
		{"comparison", `a<b`, nil},
		{"shift", `a<<b`, nil},
		{"shift assignment", `a<<=b`, nil},
		{"loop condition", `for i := 0; i<n; i++ {}`, nil},
		{"after a call", `f(x)<y`, nil},
		{"after an index", `arr[i]<n`, nil},
		{"after a literal", `"s"<t`, nil},

		// A `<` after an operator, a keyword or nothing at all is a tag.
		{"start of input", `<div/>`, []string{"<div/>"}},
		{"after return", `return <div/>`, []string{"<div/>"}},
		{"after assignment", `x:=<div/>`, []string{"<div/>"}},
		{"after an open paren", `f(<div/>)`, []string{"<div/>"}},
		{"in a composite literal", `[]Node{<div/>}`, []string{"<div/>"}},
		{"after &&", `ok && <div/>`, []string{"<div/>"}},
		{"fragment after return", `return <></>`, []string{"<></>"}},

		// Both rules meet inside one attribute splice: the splice is Go, the
		// tag around it is markup.
		{"comparison inside an attribute", `<p class={m[a<b]}>hi</p>`, []string{"<p", ">", "</p>"}},

		// Child position keeps the loose rule, where a `<` can only be markup.
		{"child position", `<p>a<b>c</b></p>`, []string{"<p>", "<b>", "</b></p>"}},

		// parse reads the raw bytes before the `<`, so a comment wedged between
		// the operand and the operator reads as a tag there. Colouring it as a
		// comparison here would hide why the file does not build.
		{"comment before the operator", `a /* c */ <b`, []string{"<b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, tok := range Tokens(tt.src) {
				if tok.Class == ClassTag {
					got = append(got, tok.Text)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("%q: tag tokens = %q, want %q", tt.src, got, tt.want)
			}
			if want := len(tt.want) > 0; parseSeesTag(tt.src) != want {
				t.Errorf("%q: parse sees a tag = %v, highlight = %v; the two must agree",
					tt.src, !want, want)
			}
		})
	}
}

// parseSeesTag reports whether the parser reads a tag in src: either it pulled
// one out, or it failed trying. The sources above are otherwise well formed, so
// an error can only come from tag parsing.
func parseSeesTag(src string) bool {
	_, tags, err := parse.RewriteTags("snippet.gsx", []byte(src))
	return len(tags) > 0 || err != nil
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
