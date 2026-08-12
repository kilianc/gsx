package e2e

import "fmt"

func init() {
	GSXFunctions["go_code"] = func() Node {
		classSeq = 0
		return GoCode()
	}
}

func GoCode() Node {
	hello := "world"
	fmt.Println(hello)

	colors := []string{"red", "green", "blue"}
	for i, color := range colors {
		colors[i] = color + "!"
	}

	lis := listItems(colors)

	topClass := "top"
	top := (
		<div class={topClass}>
		  <p>hello</p>
		  <p>{hello}</p>
		  <ul>{lis}</ul>
		</div>
	)

	bottomClass := "bottom"
	bottom := (
		<div class={bottomClass}>
		  <p class={nextClass()}>hello</p>
		  <p class={nextClass()}>{hello}</p>
		  <ul class={nextClass()}>{lis}</ul>
		</div>
	)

	return <div>{top}{bottom}</div>
}

// classSeq keeps this fixture's output deterministic: the point is to exercise a
// plain Go function call in an attribute expression, not randomness.
var classSeq int

func nextClass() string {
	classSeq++
	return fmt.Sprintf("class-%d", classSeq)
}

func listItems(colors []string) []Node {
	var lis []Node
	for _, color := range colors {
		lis = append(lis, <li>{color}</li>)
	}
	return lis
}
