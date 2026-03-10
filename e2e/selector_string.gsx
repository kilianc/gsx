package e2e

import (
	"github.com/kilianc/gsx/e2e/helpers"
)

func SelectorString() Node {
	// non-stdlib selector returning string — must be wrapped in Text
	label := helpers.MakeString("hello")

	// non-stdlib selector returning Node — must NOT be wrapped
	child := helpers.MakeNode("world")

	return (
		<div>
			<span>{label}</span>
			<span>{helpers.MakeString("inline")}</span>
			{child}
			{helpers.MakeNode("direct")}
		</div>
	)
}
