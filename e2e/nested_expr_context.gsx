package e2e

import "github.com/kilianc/gsx/e2e/helpers"

func init() {
	GSXFunctions["nested_expr_context"] = func() Node {
		return NestedExprContext(true)
	}
}

// A tag nested inside a `{...}` splice must be lowered with the same type
// context as the tag it sits in. It used to be lowered against an empty
// context, so a Node-typed variable became Text(node) — which does not compile.
func NestedExprContext(show bool) Node {
	node := Text("node-typed-var")
	nodes := []Node{Text("a"), Text("b")}
	str := "string-typed-var"

	return (
		<div>
			{If(show, <p>{node}</p>)}
			{If(show, <ul>{nodes}</ul>)}
			{If(show, <span>{str}</span>)}
			{If(show, <em>{helpers.MakeNode("from-pkg")}</em>)}
			{If(show, <b>{helpers.MakeString("str-from-pkg")}</b>)}
		</div>
	)
}
