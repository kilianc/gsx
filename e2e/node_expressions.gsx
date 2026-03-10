package e2e

import (
	"fmt"

	"github.com/kilianc/gsx/e2e/helpers"
)

func init() {
	GSXFunctions["node_expressions"] = func() Node {
		return NodeExpressions()
	}
}

func NodeExpressions() Node {
	// := from selector call returning Node (was wrapped in Text)
	child := helpers.MakeNode("hello")

	// := from local uppercase call returning Node (was wrapped in Text)
	local := MakeLocalNode("world")

	// strings must still be wrapped in Text
	name := "test"
	formatted := fmt.Sprintf("hi %s", name)

	return (
		<div>
			{child}
			{local}
			{helpers.MakeNode("direct")}
			{name}
			{formatted}
			{fmt.Sprintf("inline %s", name)}
		</div>
	)
}

func MakeLocalNode(s string) Node {
	return Text(s)
}
