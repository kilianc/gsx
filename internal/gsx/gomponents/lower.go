package gomponents

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	gotoken "go/token"

	"github.com/kilianc/gsx/internal/gsx/ast"
)

type Context struct {
	VarTypes map[string]string // Go type strings, e.g. "string", "[]string", "Node"
}

// LowerNodes lowers a list of GSX nodes to a single Go expression that evaluates to Node.
func LowerNodes(nodes []ast.Node, ctx Context) (goast.Expr, error) {
	if len(nodes) == 0 {
		return goast.NewIdent("nil"), nil
	}
	if len(nodes) == 1 {
		return lowerNode(nodes[0], ctx)
	}
	var elts []goast.Expr
	for _, n := range nodes {
		ex, err := lowerNode(n, ctx)
		if err != nil {
			return nil, err
		}
		elts = append(elts, ex)
	}
	return &goast.CompositeLit{
		Type: goast.NewIdent("Group"),
		Elts: elts,
	}, nil
}

func lowerNode(n ast.Node, ctx Context) (goast.Expr, error) {
	switch t := n.(type) {
	case ast.Text:
		return call(goast.NewIdent("Text"), strLit(t.Value)), nil
	case ast.Expr:
		ex, err := parser.ParseExpr(t.Src)
		if err != nil {
			return nil, fmt.Errorf("invalid expression %q: %w", t.Src, err)
		}

		// If this is a local identifier and we know it is a Node, splice it as-is.
		if id, ok := ex.(*goast.Ident); ok {
			if ctx.VarTypes != nil {
				if typ, ok := ctx.VarTypes[id.Name]; ok && typ == "Node" {
					return id, nil
				}
				// If this is a slice of Nodes, splice it as a grouped node list.
				if typ, ok := ctx.VarTypes[id.Name]; ok && typ == "[]Node" {
					return call(goast.NewIdent("Group"), id), nil
				}
			}
		}

		// If this looks like it produces a Node, splice it directly into children.
		// This enables patterns like `{If(cond, <p>...</p>)}` and `{Group(nodes)}`.
		if isLikelyNodeExpr(ex) {
			return ex, nil
		}
		// No implicit stringification: let the Go compiler surface a clear type error
		// if expr isn't a string.
		return call(goast.NewIdent("Text"), ex), nil
	case ast.Element:
		return lowerElement(t, ctx)
	default:
		return nil, fmt.Errorf("unsupported node type %T", n)
	}
}

func isLikelyNodeExpr(ex goast.Expr) bool {
	// Conservative heuristic: calls to identifiers starting with an uppercase letter are assumed
	// to return Node (Div/P/El/If/Group/MyComponent/etc).
	call, ok := ex.(*goast.CallExpr)
	if !ok {
		return false
	}
	switch fun := call.Fun.(type) {
	case *goast.Ident:
		if fun.Name == "" {
			return false
		}
		b := fun.Name[0]
		return b >= 'A' && b <= 'Z'
	default:
		return false
	}
}

func lowerElement(el ast.Element, ctx Context) (goast.Expr, error) {
	var args []goast.Expr

	// attrs first
	for _, a := range el.Attrs {
		ax, err := lowerAttr(a, ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, ax)
	}
	// then children
	for _, c := range el.Children {
		cx, err := lowerNode(c, ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, cx)
	}

	if fn := htmlElementFunc(el.Tag); fn != "" {
		return call(goast.NewIdent(fn), args...), nil
	}
	allArgs := append([]goast.Expr{strLit(el.Tag)}, args...)
	return call(goast.NewIdent("El"), allArgs...), nil
}

func lowerAttr(a ast.Attr, ctx Context) (goast.Expr, error) {
	switch a.Kind {
	case ast.AttrBool:
		if fn := htmlBoolAttrFunc(a.Key); fn != "" {
			return call(goast.NewIdent(fn)), nil
		}
		return call(goast.NewIdent("Attr"), strLit(a.Key)), nil
	case ast.AttrString:
		if fn := htmlStringAttrFunc(a.Key); fn != "" {
			return call(goast.NewIdent(fn), strLit(a.Value)), nil
		}
		return call(goast.NewIdent("Attr"), strLit(a.Key), strLit(a.Value)), nil
	case ast.AttrExpr:
		ex, err := parser.ParseExpr(a.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid attribute expression %q: %w", a.Value, err)
		}

		// Attribute-node injection: `{expr}` in a start tag is represented as an AttrExpr with empty Key.
		// Treat it as already-producing an attribute Node.
		if a.Key == "" {
			return ex, nil
		}

		// Allow attribute expressions that directly yield an attribute Node, e.g.
		// `data-id={If(showID, Attr("data-id", strconv.Itoa(id)))}`.
		if ce, ok := ex.(*goast.CallExpr); ok {
			if id, ok := ce.Fun.(*goast.Ident); ok {
				if id.Name == "If" || id.Name == "Iff" {
					return ex, nil
				}
			}
		}

		// Special-case class: allow either string (wrapped in Class(...)) or an attribute node
		// (e.g. components.Classes) for ergonomic conditional class patterns.
		if a.Key == "class" {
			// If we can prove it's string-ish, wrap in Class(...).
			if s, ok := lowerStringExpr(ex, ctx); ok {
				return call(goast.NewIdent("Class"), s), nil
			}
			// If it's a known Node identifier, pass through.
			if id, ok := ex.(*goast.Ident); ok {
				if ctx.VarTypes != nil {
					if typ, ok := ctx.VarTypes[id.Name]; ok && typ == "Node" {
						return id, nil
					}
				}
			}
			// If it looks like it yields an attribute node (JoinAttrs/If/Class/etc), pass through.
			if isLikelyNodeExpr(ex) {
				return ex, nil
			}
			// Default: treat as string expr and let Go typecheck it.
			return call(goast.NewIdent("Class"), ex), nil
		}

		// If this is a boolean attribute, treat `<input disabled={cond}>` like JSX:
		// include the attribute node only when cond is true.
		if fn := htmlBoolAttrFunc(a.Key); fn != "" {
			return call(goast.NewIdent("If"), ex, call(goast.NewIdent(fn))), nil
		}

		// Otherwise it's a string-ish attribute. We do not auto-coerce; let Go typecheck it.
		strExpr := ex

		if fn := htmlStringAttrFunc(a.Key); fn != "" {
			return call(goast.NewIdent(fn), strExpr), nil
		}
		return call(goast.NewIdent("Attr"), strLit(a.Key), strExpr), nil
	default:
		return nil, fmt.Errorf("unknown attr kind %v", a.Kind)
	}
}

func lowerStringExpr(ex goast.Expr, ctx Context) (goast.Expr, bool) {
	// identifier with known type
	if id, ok := ex.(*goast.Ident); ok {
		if t, ok := ctx.VarTypes[id.Name]; ok && t == "string" {
			return id, true
		}
	}

	// string literal
	if bl, ok := ex.(*goast.BasicLit); ok && bl.Kind == gotoken.STRING {
		return ex, true
	}

	// fmt.Sprintf(...) returns string
	if ce, ok := ex.(*goast.CallExpr); ok {
		if sel, ok := ce.Fun.(*goast.SelectorExpr); ok {
			if x, ok := sel.X.(*goast.Ident); ok && x.Name == "fmt" && sel.Sel != nil {
				switch sel.Sel.Name {
				case "Sprintf":
					return ex, true
				}
			}
		}
	}

	return nil, false
}

func call(fun goast.Expr, args ...goast.Expr) *goast.CallExpr {
	return &goast.CallExpr{Fun: fun, Args: args}
}

func strLit(s string) goast.Expr {
	return &goast.BasicLit{Kind: gotoken.STRING, Value: fmt.Sprintf("%q", s)}
}

func htmlElementFunc(tag string) string {
	switch tag {
	case "a":
		return "A"
	case "button":
		return "Button"
	case "div":
		return "Div"
	case "footer":
		return "Footer"
	case "form":
		return "Form"
	case "h1":
		return "H1"
	case "h2":
		return "H2"
	case "h3":
		return "H3"
	case "h4":
		return "H4"
	case "h5":
		return "H5"
	case "h6":
		return "H6"
	case "header":
		return "Header"
	case "img":
		return "Img"
	case "input":
		return "Input"
	case "label":
		return "Label"
	case "li":
		return "Li"
	case "main":
		return "Main"
	case "nav":
		return "Nav"
	case "p":
		return "P"
	case "section":
		return "Section"
	case "span":
		return "Span"
	case "ul":
		return "Ul"
	default:
		return ""
	}
}

func htmlStringAttrFunc(key string) string {
	switch key {
	case "class":
		return "Class"
	case "href":
		return "Href"
	case "id":
		return "ID"
	case "src":
		return "Src"
	case "style":
		return "Style"
	default:
		return ""
	}
}

func htmlBoolAttrFunc(key string) string {
	switch key {
	case "checked":
		return "Checked"
	case "disabled":
		return "Disabled"
	case "required":
		return "Required"
	case "selected":
		return "Selected"
	default:
		return ""
	}
}

