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
