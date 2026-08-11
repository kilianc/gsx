package gomponents

import (
	"bytes"
	goast "go/ast"
	"go/format"
	gotoken "go/token"
	"testing"

	"github.com/kilianc/gsx/internal/gsx/ast"
)

func render(t *testing.T, e goast.Expr) string {
	t.Helper()
	var buf bytes.Buffer
	if err := format.Node(&buf, gotoken.NewFileSet(), e); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// A dotted tag must lower to a real selector chain. Splitting on the first dot
// produced SelectorExpr{X: ui, Sel: Ident("widgets.Card")} — an identifier whose
// name contains a dot. That prints correctly but is not a valid tree, so any
// walk over it (import detection, html qualification) sees the wrong shape.
func TestTagFuncBuildsSelectorChain(t *testing.T) {
	fun := tagFunc("ui.widgets.Card")

	if got := render(t, fun); got != "ui.widgets.Card" {
		t.Errorf("printed as %q", got)
	}

	outer, ok := fun.(*goast.SelectorExpr)
	if !ok {
		t.Fatalf("got %T, want *ast.SelectorExpr", fun)
	}
	if outer.Sel.Name != "Card" {
		t.Errorf("outer Sel = %q, want Card", outer.Sel.Name)
	}
	inner, ok := outer.X.(*goast.SelectorExpr)
	if !ok {
		t.Fatalf("outer.X is %T, want *ast.SelectorExpr", outer.X)
	}
	if inner.Sel.Name != "widgets" {
		t.Errorf("inner Sel = %q, want widgets", inner.Sel.Name)
	}
	if id, ok := inner.X.(*goast.Ident); !ok || id.Name != "ui" {
		t.Errorf("inner.X = %#v, want Ident(ui)", inner.X)
	}
}

func TestTagFuncSingleIdent(t *testing.T) {
	if _, ok := tagFunc("Card").(*goast.Ident); !ok {
		t.Errorf("undotted tag should lower to a plain identifier")
	}
}

func TestLowerFragment(t *testing.T) {
	tests := []struct {
		name string
		node ast.Fragment
		want string
	}{
		{
			name: "empty",
			node: ast.Fragment{},
			want: "Group{}",
		},
		{
			name: "children",
			node: ast.Fragment{Children: []ast.Node{
				ast.Text{Value: "a"},
				ast.Element{Tag: "p", Children: []ast.Node{ast.Text{Value: "b"}}},
			}},
			want: `Group{Text("a"), html.P(Text("b"))}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lowerNode(tt.node, Context{HTMLPrefix: "html"})
			if err != nil {
				t.Fatal(err)
			}
			if s := render(t, got); s != tt.want {
				t.Errorf("got %s, want %s", s, tt.want)
			}
		})
	}
}

func TestHTMLElementFunc(t *testing.T) {
	tests := []struct{ tag, want string }{
		{"div", "Div"},
		{"table", "Table"},
		{"thead", "THead"},
		{"blockquote", "BlockQuote"},
		{"figcaption", "FigCaption"},
		{"em", "Em"},
		{"b", "B"},
		// `data` and `style` collide with attribute names; gomponents
		// disambiguates with an El suffix and the table must pick the element.
		{"data", "DataEl"},
		{"style", "StyleEl"},
		{"title", "TitleEl"},
		// Not an HTML element gomponents wraps: falls back to El("tag", ...).
		{"my-web-component", ""},
	}
	for _, tt := range tests {
		if got := htmlElementFunc(tt.tag); got != tt.want {
			t.Errorf("htmlElementFunc(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestAttrFuncsAcceptJSXSpellings(t *testing.T) {
	stringAttrs := []struct{ key, want string }{
		{"class", "Class"},
		{"className", "Class"},
		{"CLASSNAME", "Class"},
		{"for", "For"},
		{"htmlFor", "For"},
		{"maxlength", "MaxLength"},
		{"maxLength", "MaxLength"},
		{"autoComplete", "AutoComplete"},
		{"colSpan", "ColSpan"},
		{"srcSet", "SrcSet"},
		// `cite` is both an element and an attribute; in attribute position it
		// must resolve to the attribute constructor.
		{"cite", "CiteAttr"},
		{"style", "Style"},
		// gomponents is not consistent about which side gets the suffix: `cite`
		// names the element and `CiteAttr` the attribute, but `title` names the
		// attribute and `TitleEl` the element. Deriving the tables from the
		// source rather than transcribing them is what keeps this right.
		{"title", "Title"},
		// Open-ended namespaces have no constructor and fall back to Attr().
		{"data-kind", ""},
		{"aria-label", ""},
	}
	for _, tt := range stringAttrs {
		if got := htmlStringAttrFunc(tt.key); got != tt.want {
			t.Errorf("htmlStringAttrFunc(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}

	boolAttrs := []struct{ key, want string }{
		{"disabled", "Disabled"},
		{"required", "Required"},
		{"readonly", "ReadOnly"},
		{"readOnly", "ReadOnly"},
		{"autofocus", "AutoFocus"},
		{"autoFocus", "AutoFocus"},
		{"formNoValidate", "FormNoValidate"},
		// gomponents v1.2.0 has no Open() helper, so `open` falls back to
		// Attr("open"), which renders the same.
		{"open", ""},
	}
	for _, tt := range boolAttrs {
		if got := htmlBoolAttrFunc(tt.key); got != tt.want {
			t.Errorf("htmlBoolAttrFunc(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

// htmlExports drives identifier qualification inside `{...}` splices. The
// hand-maintained version listed Open and NoValidate, which gomponents v1.2.0
// does not export, so a user identifier by either name was rewritten to a
// non-existent html.Open / html.NoValidate and failed to compile.
func TestHTMLExportsHasNoPhantoms(t *testing.T) {
	for _, name := range []string{"Open", "NoValidate"} {
		if htmlExports[name] {
			t.Errorf("htmlExports claims %q exists in gomponents/html; it does not", name)
		}
	}
	for _, name := range []string{"Div", "Class", "Disabled", "TitleEl"} {
		if !htmlExports[name] {
			t.Errorf("htmlExports is missing %q", name)
		}
	}
}

// A local declaration must shadow a gomponents/html export of the same name,
// exactly as it would in ordinary Go. Without this, a component called Section
// or Code has its call sites inside `{...}` rewritten to html.Section and
// html.Code, and the generated file does not compile.
func TestLocalNamesShadowHTMLExports(t *testing.T) {
	expr := ast.Expr{Src: `Section(Code("x"))`}

	t.Run("without local declarations", func(t *testing.T) {
		got, err := lowerNode(expr, Context{HTMLPrefix: "html"})
		if err != nil {
			t.Fatal(err)
		}
		if s := render(t, got); s != `html.Section(html.Code("x"))` {
			t.Errorf("got %s", s)
		}
	})

	t.Run("with local declarations", func(t *testing.T) {
		ctx := Context{
			HTMLPrefix: "html",
			LocalNames: map[string]bool{"Section": true, "Code": true},
		}
		got, err := lowerNode(expr, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s := render(t, got); s != `Section(Code("x"))` {
			t.Errorf("got %s, want the local declarations to win", s)
		}
	})

	t.Run("shadowing is per name", func(t *testing.T) {
		ctx := Context{
			HTMLPrefix: "html",
			LocalNames: map[string]bool{"Section": true},
		}
		got, err := lowerNode(expr, ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s := render(t, got); s != `Section(html.Code("x"))` {
			t.Errorf("got %s", s)
		}
	})
}
