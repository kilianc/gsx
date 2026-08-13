package parse

import (
	"strings"
	"testing"

	"github.com/kilianc/gsx/internal/gsx/ast"
)

func TestRewriteTagsReplacesTopLevelTags(t *testing.T) {
	src := []byte("package p\n\nfunc F() Node { return <div>hi</div> }\n")

	out, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "package p\n\nfunc F() Node { return __gsx_expr_1() }\n"; string(out) != want {
		t.Errorf("rewritten =\n%q\nwant\n%q", out, want)
	}
	if len(tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(tags))
	}
	if got := string(src[tags[0].SrcStart:tags[0].SrcEnd]); got != "<div>hi</div>" {
		t.Errorf("src span = %q, want %q", got, "<div>hi</div>")
	}
	if got := string(out[tags[0].TgtStart:tags[0].TgtEnd]); got != "__gsx_expr_1()" {
		t.Errorf("tgt span = %q", got)
	}
}

// A `<` inside a comment or a Go literal must never start a tag.
func TestRewriteTagsIgnoresNonCode(t *testing.T) {
	for _, src := range []string{
		`package p // <div>not a tag</div>`,
		"package p /* <div>not a tag</div> */",
		`package p; var s = "<div>not a tag</div>"`,
		"package p; var s = `<div>not a tag</div>`",
		`package p; var r = '<'`,
		`package p; var b = a < b`,
	} {
		out, tags, err := RewriteTags("f.gsx", []byte(src))
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if len(tags) != 0 {
			t.Errorf("%s: found %d tags, want 0", src, len(tags))
		}
		if string(out) != src {
			t.Errorf("%s: rewritten to %q, want unchanged", src, out)
		}
	}
}

func TestParseNodePositions(t *testing.T) {
	src := []byte(`package p
func F() Node {
	return <div class="c">text{expr}</div>
}
`)
	_, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)

	// Every position must point at the exact byte the construct starts on.
	assertSpanStartsWith(t, src, el.Off(), "<div")
	assertSpanStartsWith(t, src, el.Attrs[0].Pos, `class="c"`)
	assertSpanStartsWith(t, src, el.Children[0].Off(), "text")
	assertSpanStartsWith(t, src, el.Children[1].Off(), "{expr}")
}

func assertSpanStartsWith(t *testing.T, src []byte, off int, want string) {
	t.Helper()
	if off < 0 || off > len(src) {
		t.Errorf("offset %d out of range", off)
		return
	}
	if got := string(src[off:]); !strings.HasPrefix(got, want) {
		t.Errorf("offset %d points at %q, want it to start %q", off, truncate(got, len(want)+10), want)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Tags inside a `{...}` splice become placeholder calls so they can be lowered
// later with the enclosing tag's type context.
func TestNestedTagsBecomePlaceholders(t *testing.T) {
	src := []byte(`package p
func F() Node { return <div>{If(ok, <p>hi</p>)}</div> }
`)
	_, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	expr, ok := el.Children[0].(ast.Expr)
	if !ok {
		t.Fatalf("child 0 is %T, want ast.Expr", el.Children[0])
	}
	if want := "If(ok, __gsx_sub_1())"; expr.Src != want {
		t.Errorf("Src = %q, want %q", expr.Src, want)
	}
	nested, ok := expr.Nested["__gsx_sub_1"]
	if !ok {
		t.Fatalf("Nested has no __gsx_sub_1, got %v", expr.Nested)
	}
	if got := nested.(ast.Element).Tag; got != "p" {
		t.Errorf("nested tag = %q, want %q", got, "p")
	}
	// The nested node's offset must still refer to the original file.
	assertSpanStartsWith(t, src, nested.Off(), "<p>hi</p>")
}

func TestNestedPlaceholdersAreUniqueAcrossTags(t *testing.T) {
	src := []byte(`package p
func F() Node { return <div>{A(<p>1</p>)}{B(<p>2</p>)}</div> }
`)
	_, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	seen := map[string]bool{}
	for _, c := range el.Children {
		for name := range c.(ast.Expr).Nested {
			if seen[name] {
				t.Errorf("placeholder %q reused", name)
			}
			seen[name] = true
		}
	}
	if len(seen) != 2 {
		t.Errorf("got %d placeholders, want 2", len(seen))
	}
}

func TestBracedExprHandlesNestedBracesAndLiterals(t *testing.T) {
	src := []byte(`package p
func F() Node { return <div>{f(map[string]int{"a": 1}, "}", ` + "`}`" + `)}</div> }
`)
	_, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	want := `f(map[string]int{"a": 1}, "}", ` + "`}`" + `)`
	if got := el.Children[0].(ast.Expr).Src; got != want {
		t.Errorf("Src = %q, want %q", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantMsg  string
		wantLine int
		wantCol  int
	}{
		{
			name:     "mismatched closing tag",
			src:      "package p\nfunc F() Node { return <div>hi</span> }\n",
			wantMsg:  "mismatched closing tag </span>, expected </div>",
			wantLine: 2,
			wantCol:  31,
		},
		{
			name:     "unclosed element",
			src:      "package p\nfunc F() Node { return <div>hi }\n",
			wantMsg:  "unclosed <div>: reached end of file without a matching </div>",
			wantLine: 2,
			wantCol:  24,
		},
		{
			name:     "unterminated brace",
			src:      "package p\nfunc F() Node { return <div>{oops</div>\n",
			wantMsg:  "unterminated `{`: reached end of file without a matching `}`",
			wantLine: 2,
			wantCol:  29,
		},
		{
			// A `{` that swallows the close tag reports the element as unclosed
			// and points at the element, which is where the fix goes.
			name:     "brace swallows close tag",
			src:      "package p\nfunc F() Node { return <div>{oops</div> }\n",
			wantMsg:  "unclosed <div>: reached end of file without a matching </div>",
			wantLine: 2,
			wantCol:  24,
		},
		{
			name:     "question-mark attribute",
			src:      "package p\nfunc F() Node { return <div hidden?={x}></div> }\n",
			wantMsg:  "unsupported `?=` syntax",
			wantLine: 2,
			wantCol:  35,
		},
		{
			name:     "missing attribute value",
			src:      "package p\nfunc F() Node { return <div class=x></div> }\n",
			wantMsg:  "expected `\"` or `{` after `class=` in <div>",
			wantLine: 2,
			wantCol:  35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := RewriteTags("page.gsx", []byte(tt.src))
			if err == nil {
				t.Fatal("got nil error, want a parse error")
			}
			pe, ok := err.(*Error)
			if !ok {
				t.Fatalf("got %T, want *parse.Error", err)
			}
			if !strings.Contains(pe.Msg, tt.wantMsg) {
				t.Errorf("Msg = %q, want it to contain %q", pe.Msg, tt.wantMsg)
			}
			line, col := pe.Position()
			if line != tt.wantLine || col != tt.wantCol {
				t.Errorf("position = %d:%d, want %d:%d", line, col, tt.wantLine, tt.wantCol)
			}
			// The rendered error must be actionable: path, position, and a caret.
			s := pe.Error()
			for _, want := range []string{"page.gsx:", "|", "^"} {
				if !strings.Contains(s, want) {
					t.Errorf("rendered error missing %q:\n%s", want, s)
				}
			}
		})
	}
}

func TestErrorRendering(t *testing.T) {
	src := "package p\nfunc F() Node {\n\treturn <div>hi</span>\n}\n"
	_, _, err := RewriteTags("page.gsx", []byte(src))
	if err == nil {
		t.Fatal("want error")
	}
	want := "page.gsx:3:16: mismatched closing tag </span>, expected </div>\n\n" +
		"  3 |  return <div>hi</span>\n" +
		"    |                ^"
	if got := err.Error(); got != want {
		t.Errorf("got:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestLineCol(t *testing.T) {
	src := []byte("ab\ncd\n")
	for _, tt := range []struct {
		off       int
		line, col int
	}{
		{0, 1, 1},
		{1, 1, 2},
		{2, 1, 3}, // the newline itself
		{3, 2, 1},
		{4, 2, 2},
		{6, 3, 1}, // end of input
	} {
		line, col := LineCol(src, tt.off)
		if line != tt.line || col != tt.col {
			t.Errorf("LineCol(%d) = %d:%d, want %d:%d", tt.off, line, col, tt.line, tt.col)
		}
	}
}

func TestParseFragment(t *testing.T) {
	src := []byte("package p\nfunc F() Node { return <><h1>a</h1><p>b</p></> }\n")

	out, tags, err := RewriteTags("f.gsx", src)
	if err != nil {
		t.Fatal(err)
	}
	if want := "package p\nfunc F() Node { return __gsx_expr_1() }\n"; string(out) != want {
		t.Errorf("rewritten = %q, want %q", out, want)
	}
	frag, ok := tags[0].Node.(ast.Fragment)
	if !ok {
		t.Fatalf("got %T, want ast.Fragment", tags[0].Node)
	}
	if len(frag.Children) != 2 {
		t.Fatalf("got %d children, want 2", len(frag.Children))
	}
	assertSpanStartsWith(t, src, frag.Off(), "<>")
}

func TestParseEmptyFragment(t *testing.T) {
	_, tags, err := RewriteTags("f.gsx", []byte("package p\nfunc F() Node { return <></> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	frag, ok := tags[0].Node.(ast.Fragment)
	if !ok {
		t.Fatalf("got %T, want ast.Fragment", tags[0].Node)
	}
	if len(frag.Children) != 0 {
		t.Errorf("got %d children, want 0", len(frag.Children))
	}
}

func TestNestedFragment(t *testing.T) {
	_, tags, err := RewriteTags("f.gsx", []byte("package p\nfunc F() Node { return <div><><p>a</p></></div> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	if _, ok := el.Children[0].(ast.Fragment); !ok {
		t.Fatalf("child 0 is %T, want ast.Fragment", el.Children[0])
	}
}

func TestFragmentCloseMismatch(t *testing.T) {
	_, _, err := RewriteTags("f.gsx", []byte("package p\nfunc F() Node { return <><p>a</p></div> }\n"))
	if err == nil {
		t.Fatal("want error")
	}
	if want := "mismatched closing tag </div>, expected </>"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
}

// `<` must still only start a tag when followed by a name or `>`, so ordinary
// Go comparisons and channel receives are untouched.
func TestFragmentDetectionDoesNotBreakGo(t *testing.T) {
	for _, src := range []string{
		"package p; var x = a < b",
		"package p; var x = a <- b",
		"package p; var x = a << b",
		"package p; func f() { v := <-ch; _ = v }",
		// Unspaced, the operand before the `<` is the only thing separating a
		// comparison from a `<b>` tag.
		"package p; var x = a<b",
		"package p; var x = a<<b",
		"package p; var x = a<=b",
		"package p; var x = a<<=b",
		"package p; func f() { for i := 0; i<n; i++ { g() } }",
		"package p; var x = f(y)<n",
		"package p; var x = m[k]<n",
		"package p; var x = a /* c */ <b",
	} {
		out, tags, err := RewriteTags("f.gsx", []byte(src))
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if len(tags) != 0 || string(out) != src {
			t.Errorf("%s: rewritten to %q with %d tags", src, out, len(tags))
		}
	}
}

// A splice is Go code, so the same rule applies inside one: `a<b` there is a
// comparison and must reach the compiler untouched.
func TestComparisonInsideSplice(t *testing.T) {
	_, tags, err := RewriteTags("f.gsx", []byte(
		"package p\nfunc F() Node { return <p class={m[a<b]}>{i<n}</p> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	if got := el.Attrs[0].Value; got != "m[a<b]" {
		t.Errorf("attr value = %q, want %q", got, "m[a<b]")
	}
	if len(el.Attrs[0].Nested) != 0 {
		t.Errorf("attr has %d nested tags, want 0", len(el.Attrs[0].Nested))
	}
	if got := el.Children[0].(ast.Expr).Src; got != "i<n" {
		t.Errorf("child expr = %q, want %q", got, "i<n")
	}
}

// The other half of the rule: after an operator, a keyword or a `{`, a `<` is
// still a tag even with no space in front of it.
func TestTagStartsAfterANonOperand(t *testing.T) {
	for _, src := range []string{
		"package p\nfunc F() Node { return <div/> }\n",
		"package p\nfunc F() Node { return(<div/>) }\n",
		"package p\nvar x = []Node{<div/>}\n",
		"package p\nfunc F() Node { return <p>{ok && <div/>}</p> }\n",
		"package p\nfunc F() Node { return /* c */ <div/> }\n",
	} {
		_, tags, err := RewriteTags("f.gsx", []byte(src))
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if len(tags) != 1 {
			t.Errorf("%s: got %d tags, want 1", src, len(tags))
		}
	}
}

func TestJSXCommentsAreDropped(t *testing.T) {
	_, tags, err := RewriteTags("f.gsx", []byte(
		"package p\nfunc F() Node { return <div {/* attr note */} class=\"c\">{/* child note */}<p>a</p></div> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	if len(el.Attrs) != 1 || el.Attrs[0].Key != "class" {
		t.Errorf("attrs = %+v, want only class", el.Attrs)
	}
	if len(el.Children) != 1 {
		t.Fatalf("got %d children, want 1", len(el.Children))
	}
	if _, ok := el.Children[0].(ast.Element); !ok {
		t.Errorf("child 0 is %T, want ast.Element", el.Children[0])
	}
}

func TestTextIsNormalizedAndDecoded(t *testing.T) {
	_, tags, err := RewriteTags("f.gsx", []byte(
		"package p\nfunc F() Node { return <p>\n\tTom &amp; Jerry\n\tsecond line\n</p> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	el := tags[0].Node.(ast.Element)
	got := el.Children[0].(ast.Text).Value
	if want := "Tom & Jerry second line"; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}
