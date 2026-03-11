package gomponents

import (
	"fmt"
	goast "go/ast"
	"go/parser"
	gotoken "go/token"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/ast"
)

type FuncParam struct {
	Name     string
	Type     string // "string", "bool", "int", "Node", etc.
	Variadic bool
}

type Context struct {
	VarTypes        map[string]string      // Go type strings, e.g. "string", "[]string", "Node"
	FuncReturnTypes map[string]string      // "pkg.Func" → "string" | "Node", resolved via go/importer
	FuncParams      map[string][]FuncParam // "pkg.Func" → ordered param list
	HTMLPrefix      string                 // when non-empty, qualify gomponents/html calls (e.g. "html")
}

func (ctx Context) htmlIdent(name string) goast.Expr {
	if ctx.HTMLPrefix != "" {
		return &goast.SelectorExpr{
			X:   goast.NewIdent(ctx.HTMLPrefix),
			Sel: goast.NewIdent(name),
		}
	}
	return goast.NewIdent(name)
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
				if typ, ok := ctx.VarTypes[id.Name]; ok && typ == "[]Node" {
					return call(goast.NewIdent("Group"), id), nil
				}
			}
		}

		// Check selector expressions (e.g. d.Child) against known struct field types.
		if sel, ok := ex.(*goast.SelectorExpr); ok {
			if xID, ok := sel.X.(*goast.Ident); ok && ctx.VarTypes != nil {
				key := xID.Name + "." + sel.Sel.Name
				if typ, ok := ctx.VarTypes[key]; ok && typ == "Node" {
					return ex, nil
				}
				if typ, ok := ctx.VarTypes[key]; ok && typ == "[]Node" {
					return call(goast.NewIdent("Group"), ex), nil
				}
			}
		}

		// If this looks like it produces a Node, splice it directly into children.
		// This enables patterns like `{If(cond, <p>...</p>)}` and `{Group(nodes)}`.
		if IsLikelyNodeExpr(ex, ctx) {
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

func IsLikelyNodeExpr(ex goast.Expr, ctx Context) bool {
	call, ok := ex.(*goast.CallExpr)
	if !ok {
		return false
	}
	switch fun := call.Fun.(type) {
	case *goast.Ident:
		if fun.Name == "" {
			return false
		}
		if ctx.FuncReturnTypes != nil {
			if typ, ok := ctx.FuncReturnTypes[fun.Name]; ok {
				return typ == "Node"
			}
		}
		return fun.Name[0] >= 'A' && fun.Name[0] <= 'Z'
	case *goast.SelectorExpr:
		xID, ok := fun.X.(*goast.Ident)
		if !ok {
			return false
		}
		if IsKnownNonNodePkg(xID.Name) {
			return false
		}
		if ctx.FuncReturnTypes != nil {
			if typ, ok := ctx.FuncReturnTypes[xID.Name+"."+fun.Sel.Name]; ok {
				return typ == "Node"
			}
		}
		return true
	default:
		return false
	}
}

func IsExprStringish(e goast.Expr) bool {
	if bl, ok := e.(*goast.BasicLit); ok && bl.Kind == gotoken.STRING {
		return true
	}
	if ce, ok := e.(*goast.CallExpr); ok {
		if sel, ok := ce.Fun.(*goast.SelectorExpr); ok {
			if x, ok := sel.X.(*goast.Ident); ok && sel.Sel != nil {
				switch {
				case x.Name == "fmt" && sel.Sel.Name == "Sprintf":
					return true
				case x.Name == "strconv" && sel.Sel.Name == "Itoa":
					return true
				}
			}
		}
	}
	return false
}

func IsKnownNonNodePkg(name string) bool {
	switch name {
	case "fmt", "strconv", "strings", "path", "filepath", "time", "os",
		"math", "regexp", "sort", "bytes", "unicode", "encoding",
		"errors", "log", "sync", "io", "net", "context", "reflect":
		return true
	}
	return false
}

func lowerElement(el ast.Element, ctx Context) (goast.Expr, error) {
	// Components with typed params (not pure ...Node) use named-param mapping
	// instead of wrapping attribute values in Attr()/Class()/etc.
	if ctx.FuncParams != nil {
		if params, ok := ctx.FuncParams[el.Tag]; ok && hasTypedParams(params) {
			var fun goast.Expr
			if dot := strings.IndexByte(el.Tag, '.'); dot >= 0 {
				fun = &goast.SelectorExpr{
					X:   goast.NewIdent(el.Tag[:dot]),
					Sel: goast.NewIdent(el.Tag[dot+1:]),
				}
			} else {
				fun = goast.NewIdent(el.Tag)
			}
			return lowerTypedComponent(el, fun, params, ctx)
		}
	}

	var args []goast.Expr

	for _, a := range el.Attrs {
		ax, err := lowerAttr(a, ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, ax)
	}
	for _, c := range el.Children {
		cx, err := lowerNode(c, ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, cx)
	}

	if dot := strings.IndexByte(el.Tag, '.'); dot >= 0 {
		fun := &goast.SelectorExpr{
			X:   goast.NewIdent(el.Tag[:dot]),
			Sel: goast.NewIdent(el.Tag[dot+1:]),
		}
		return call(fun, args...), nil
	}
	if len(el.Tag) > 0 && el.Tag[0] >= 'A' && el.Tag[0] <= 'Z' {
		return call(goast.NewIdent(el.Tag), args...), nil
	}

	if fn := htmlElementFunc(el.Tag); fn != "" {
		return call(ctx.htmlIdent(fn), args...), nil
	}
	allArgs := append([]goast.Expr{strLit(el.Tag)}, args...)
	return call(goast.NewIdent("El"), allArgs...), nil
}

// hasTypedParams reports whether the param list contains any non-Node types,
// meaning the function expects typed arguments rather than variadic Node children.
func hasTypedParams(params []FuncParam) bool {
	for _, p := range params {
		if p.Type != "Node" && p.Type != "" {
			return true
		}
	}
	return false
}

// lowerTypedComponent handles components whose function signature has typed parameters.
// Attributes are mapped to positional arguments by matching param names.
// Any unmatched attributes and all children are passed to a trailing variadic ...Node param.
func lowerTypedComponent(el ast.Element, fun goast.Expr, params []FuncParam, ctx Context) (goast.Expr, error) {
	hasVariadic := len(params) > 0 && params[len(params)-1].Variadic
	namedParams := params
	if hasVariadic {
		namedParams = params[:len(params)-1]
	}

	paramIdx := map[string]int{}
	for i, p := range namedParams {
		paramIdx[p.Name] = i
	}

	positional := make([]goast.Expr, len(namedParams))
	used := make([]bool, len(namedParams))
	var variadicArgs []goast.Expr

	for _, a := range el.Attrs {
		if idx, ok := paramIdx[a.Key]; ok {
			val, err := lowerAttrValue(a)
			if err != nil {
				return nil, err
			}
			positional[idx] = val
			used[idx] = true
		} else if hasVariadic {
			ax, err := lowerAttr(a, ctx)
			if err != nil {
				return nil, err
			}
			variadicArgs = append(variadicArgs, ax)
		}
	}

	for _, c := range el.Children {
		cx, err := lowerNode(c, ctx)
		if err != nil {
			return nil, err
		}
		variadicArgs = append(variadicArgs, cx)
	}

	var args []goast.Expr
	for i := range namedParams {
		if used[i] {
			args = append(args, positional[i])
		}
	}
	args = append(args, variadicArgs...)

	return call(fun, args...), nil
}

// lowerAttrValue converts an attribute to a raw Go expression (not wrapped in Attr/Class/etc).
func lowerAttrValue(a ast.Attr) (goast.Expr, error) {
	switch a.Kind {
	case ast.AttrBool:
		return goast.NewIdent("true"), nil
	case ast.AttrString:
		return strLit(a.Value), nil
	case ast.AttrExpr:
		return parser.ParseExpr(a.Value)
	default:
		return nil, fmt.Errorf("unknown attr kind %v", a.Kind)
	}
}

func lowerAttr(a ast.Attr, ctx Context) (goast.Expr, error) {
	switch a.Kind {
	case ast.AttrBool:
		if fn := htmlBoolAttrFunc(a.Key); fn != "" {
			return call(ctx.htmlIdent(fn)), nil
		}
		return call(goast.NewIdent("Attr"), strLit(a.Key)), nil
	case ast.AttrString:
		if fn := htmlStringAttrFunc(a.Key); fn != "" {
			return call(ctx.htmlIdent(fn), strLit(a.Value)), nil
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
				return call(ctx.htmlIdent("Class"), s), nil
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
			if IsLikelyNodeExpr(ex, ctx) {
				return ex, nil
			}
			// Default: treat as string expr and let Go typecheck it.
			return call(ctx.htmlIdent("Class"), ex), nil
		}

		// If this is a boolean attribute, treat `<input disabled={cond}>` like JSX:
		// include the attribute node only when cond is true.
		if fn := htmlBoolAttrFunc(a.Key); fn != "" {
			return call(goast.NewIdent("If"), ex, call(ctx.htmlIdent(fn))), nil
		}

		// Otherwise it's a string-ish attribute. We do not auto-coerce; let Go typecheck it.
		strExpr := ex

		if fn := htmlStringAttrFunc(a.Key); fn != "" {
			return call(ctx.htmlIdent(fn), strExpr), nil
		}
		return call(goast.NewIdent("Attr"), strLit(a.Key), strExpr), nil
	default:
		return nil, fmt.Errorf("unknown attr kind %v", a.Kind)
	}
}

func lowerStringExpr(ex goast.Expr, ctx Context) (goast.Expr, bool) {
	if id, ok := ex.(*goast.Ident); ok {
		if t, ok := ctx.VarTypes[id.Name]; ok && t == "string" {
			return id, true
		}
	}
	if IsExprStringish(ex) {
		return ex, true
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

















