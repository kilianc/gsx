package e2e

import (
	"fmt"
	"math/rand"
)

func init() {
	GSXFunctions["go_code"] = func() Node {
		rand.Seed(1)
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
      <p class={getRandomClass()}>hello</p>
      <p class={getRandomClass()}>{hello}</p>
      <ul class={getRandomClass()}>{lis}</ul>
    </div>
  )

	return <div>{top}{bottom}</div>
}

func getRandomClass() string {
	return fmt.Sprintf("class-%d", rand.Intn(100))
}

func listItems(colors []string) []Node {
	var lis []Node
	for _, color := range colors {
		lis = append(lis, <li>{color}</li>)
	}
	return lis
}
