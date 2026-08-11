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
		ex, err := parseSplice(t.Src, t.Nested, ctx)
		if err != nil {
			return nil, err
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
	case ast.Fragment:
		return lowerFragment(t, ctx)
	default:
		return nil, fmt.Errorf("unsupported node type %T", n)
	}
}

// lowerFragment lowers `<>...</>` to a Group, which renders its children with
// no wrapping element.
func lowerFragment(f ast.Fragment, ctx Context) (goast.Expr, error) {
	elts := make([]goast.Expr, 0, len(f.Children))
	for _, c := range f.Children {
		ex, err := lowerNode(c, ctx)
		if err != nil {
			return nil, err
		}
		elts = append(elts, ex)
	}
	// Group{} for an empty fragment renders nothing, which is what `<></>` means.
	return &goast.CompositeLit{Type: goast.NewIdent("Group"), Elts: elts}, nil
}

// tagFunc builds the callee for a component tag, turning a dotted name into a
// proper selector chain.
//
// `ui.widgets.Card` must become SelectorExpr(SelectorExpr(ui, widgets), Card).
// Splitting only on the first dot yields a selector whose Sel is an identifier
// literally named "widgets.Card"; it prints correctly but is not a valid tree,
// so every later walk over it — import detection, html qualification, nested
// substitution — sees the wrong shape.
func tagFunc(tag string) goast.Expr {
	parts := strings.Split(tag, ".")
	var fun goast.Expr = goast.NewIdent(parts[0])
	for _, part := range parts[1:] {
		fun = &goast.SelectorExpr{X: fun, Sel: goast.NewIdent(part)}
	}
	return fun
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
			return lowerTypedComponent(el, tagFunc(el.Tag), params, ctx)
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

	if strings.Contains(el.Tag, ".") {
		return call(tagFunc(el.Tag), args...), nil
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
		// An unnamed parameter cannot be addressed by an attribute, and letting
		// it into the index would let a keyless attribute — `{expr}` or
		// `{...expr}` — bind to it by accident.
		if p.Name != "" {
			paramIdx[p.Name] = i
		}
	}

	positional := make([]goast.Expr, len(namedParams))
	used := make([]bool, len(namedParams))
	var variadicArgs []goast.Expr

	for _, a := range el.Attrs {
		if idx, ok := paramIdx[a.Key]; ok && a.Key != "" {
			val, err := lowerAttrValue(a, ctx)
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
func lowerAttrValue(a ast.Attr, ctx Context) (goast.Expr, error) {
	switch a.Kind {
	case ast.AttrBool:
		return goast.NewIdent("true"), nil
	case ast.AttrString:
		return strLit(a.Value), nil
	case ast.AttrExpr:
		return parseSplice(a.Value, a.Nested, ctx)
	default:
		return nil, fmt.Errorf("unknown attr kind %v", a.Kind)
	}
}

// parseSplice parses the Go source of a `{...}` splice and resolves it into a
// single Go expression.
//
// Tag expressions nested inside the splice were replaced by placeholder calls
// during parsing. They are lowered here, with the caller's full type context, so
// that `{If(ok, <p>{n}</p>)}` sees the same VarTypes as the tag it sits in. The
// parser used to lower them eagerly against an empty context, which produced
// `Text(n)` for a Node-typed `n` — code that does not compile.
func parseSplice(src string, nested map[string]ast.Node, ctx Context) (goast.Expr, error) {
	ex, err := parser.ParseExpr(src)
	if err != nil {
		return nil, fmt.Errorf("invalid expression %q: %w", src, err)
	}
	if len(nested) > 0 {
		if ex, err = substituteNested(ex, nested, ctx); err != nil {
			return nil, err
		}
	}
	return ctx.qualifyHTMLInExpr(ex), nil
}

// substituteNested replaces each `__gsx_sub_N()` placeholder call with the
// lowered form of the tag it stands for.
func substituteNested(root goast.Expr, nested map[string]ast.Node, ctx Context) (goast.Expr, error) {
	var firstErr error

	var walk func(goast.Expr) goast.Expr
	walk = func(e goast.Expr) goast.Expr {
		if e == nil || firstErr != nil {
			return e
		}
		if ce, ok := e.(*goast.CallExpr); ok && len(ce.Args) == 0 {
			if id, ok := ce.Fun.(*goast.Ident); ok {
				if node, ok := nested[id.Name]; ok {
					lowered, err := lowerNode(node, ctx)
					if err != nil {
						firstErr = err
						return e
					}
					// Already fully lowered; do not descend into it.
					return lowered
				}
			}
		}
		rewriteChildren(e, walk)
		return e
	}

	out := walk(root)
	return out, firstErr
}

// rewriteChildren applies fn to every sub-expression of e, in place.
//
// SelectorExpr.Sel and KeyValueExpr.Key are names, not expressions to rewrite,
// so they are deliberately skipped.
func rewriteChildren(e goast.Expr, fn func(goast.Expr) goast.Expr) {
	switch t := e.(type) {
	case *goast.CallExpr:
		t.Fun = fn(t.Fun)
		for i := range t.Args {
			t.Args[i] = fn(t.Args[i])
		}
	case *goast.SelectorExpr:
		t.X = fn(t.X)
	case *goast.CompositeLit:
		for i := range t.Elts {
			t.Elts[i] = fn(t.Elts[i])
		}
	case *goast.KeyValueExpr:
		t.Value = fn(t.Value)
	case *goast.ParenExpr:
		t.X = fn(t.X)
	case *goast.UnaryExpr:
		t.X = fn(t.X)
	case *goast.StarExpr:
		t.X = fn(t.X)
	case *goast.BinaryExpr:
		t.X = fn(t.X)
		t.Y = fn(t.Y)
	case *goast.IndexExpr:
		t.X = fn(t.X)
		t.Index = fn(t.Index)
	case *goast.SliceExpr:
		t.X = fn(t.X)
		if t.Low != nil {
			t.Low = fn(t.Low)
		}
		if t.High != nil {
			t.High = fn(t.High)
		}
		if t.Max != nil {
			t.Max = fn(t.Max)
		}
	case *goast.TypeAssertExpr:
		t.X = fn(t.X)
	case *goast.FuncLit:
		rewriteChildrenInStmts(t.Body, fn)
	}
}

func rewriteChildrenInStmts(b *goast.BlockStmt, fn func(goast.Expr) goast.Expr) {
	if b == nil {
		return
	}
	for _, s := range b.List {
		switch st := s.(type) {
		case *goast.ReturnStmt:
			for i := range st.Results {
				st.Results[i] = fn(st.Results[i])
			}
		case *goast.ExprStmt:
			st.X = fn(st.X)
		case *goast.AssignStmt:
			for i := range st.Rhs {
				st.Rhs[i] = fn(st.Rhs[i])
			}
		case *goast.BlockStmt:
			rewriteChildrenInStmts(st, fn)
		case *goast.IfStmt:
			rewriteChildrenInStmts(st.Body, fn)
			if eb, ok := st.Else.(*goast.BlockStmt); ok {
				rewriteChildrenInStmts(eb, fn)
			}
		case *goast.RangeStmt:
			rewriteChildrenInStmts(st.Body, fn)
		case *goast.ForStmt:
			rewriteChildrenInStmts(st.Body, fn)
		}
	}
}

func lowerAttr(a ast.Attr, ctx Context) (goast.Expr, error) {
	switch a.Kind {
	case ast.AttrSpread:
		// `{...attrs}` where attrs is a []Node. Group turns it back into the
		// single Node an element's variadic argument list expects, so a set of
		// attributes can be built once and applied to many elements.
		ex, err := parseSplice(a.Value, a.Nested, ctx)
		if err != nil {
			return nil, err
		}
		return call(goast.NewIdent("Group"), ex), nil
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
		ex, err := parseSplice(a.Value, a.Nested, ctx)
		if err != nil {
			return nil, err
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

// qualifyHTMLInExpr walks a user-parsed Go expression and replaces bare
// identifiers that are known gomponents/html exports with html-qualified
// selector expressions (e.g. Class → html.Class).
func (ctx Context) qualifyHTMLInExpr(expr goast.Expr) goast.Expr {
	if ctx.HTMLPrefix == "" {
		return expr
	}
	return qualifyHTMLWalk(expr, ctx.HTMLPrefix)
}

func qualifyHTMLWalk(expr goast.Expr, prefix string) goast.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *goast.Ident:
		if htmlExports[e.Name] {
			return &goast.SelectorExpr{
				X:   goast.NewIdent(prefix),
				Sel: goast.NewIdent(e.Name),
			}
		}
	case *goast.CallExpr:
		e.Fun = qualifyHTMLWalk(e.Fun, prefix)
		for i, arg := range e.Args {
			e.Args[i] = qualifyHTMLWalk(arg, prefix)
		}
	case *goast.CompositeLit:
		for i, elt := range e.Elts {
			e.Elts[i] = qualifyHTMLWalk(elt, prefix)
		}
	case *goast.KeyValueExpr:
		e.Value = qualifyHTMLWalk(e.Value, prefix)
	case *goast.ParenExpr:
		e.X = qualifyHTMLWalk(e.X, prefix)
	case *goast.UnaryExpr:
		e.X = qualifyHTMLWalk(e.X, prefix)
	case *goast.BinaryExpr:
		e.X = qualifyHTMLWalk(e.X, prefix)
		e.Y = qualifyHTMLWalk(e.Y, prefix)
	case *goast.IndexExpr:
		e.X = qualifyHTMLWalk(e.X, prefix)
		e.Index = qualifyHTMLWalk(e.Index, prefix)
	case *goast.SliceExpr:
		e.X = qualifyHTMLWalk(e.X, prefix)
	case *goast.FuncLit:
		qualifyHTMLWalkStmts(e.Body.List, prefix)
	}
	return expr
}

func qualifyHTMLWalkStmts(stmts []goast.Stmt, prefix string) {
	for _, s := range stmts {
		switch st := s.(type) {
		case *goast.ReturnStmt:
			for i, r := range st.Results {
				st.Results[i] = qualifyHTMLWalk(r, prefix)
			}
		case *goast.ExprStmt:
			st.X = qualifyHTMLWalk(st.X, prefix)
		case *goast.AssignStmt:
			for i, r := range st.Rhs {
				st.Rhs[i] = qualifyHTMLWalk(r, prefix)
			}
		case *goast.BlockStmt:
			qualifyHTMLWalkStmts(st.List, prefix)
		case *goast.IfStmt:
			if st.Body != nil {
				qualifyHTMLWalkStmts(st.Body.List, prefix)
			}
			if st.Else != nil {
				if b, ok := st.Else.(*goast.BlockStmt); ok {
					qualifyHTMLWalkStmts(b.List, prefix)
				}
			}
		}
	}
}
