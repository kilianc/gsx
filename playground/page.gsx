package playground

import "fmt"

// ItemsList is a tiny helper so you can test go-to-definition/hover from a .gsx file.
func ItemsList(lis []Node) Node {
	return <ul class="items">{lis}</ul>
}

func Page(title string, items []string, show bool) Node {
	if title == "" {
		return <p>no title</p>
	}

	var lis []Node
	for _, it := range items {
		lis = append(lis, <li class="item">{it}</li>)
	}

	// Intentional error (to verify diagnostics from gopls show up in .gsx):
	// Uncomment this to see an "undefined: notDefined" error.
	// _ = notDefined

	banner := (
		<div class="page">
			<h1>{title}</h1>
			{If(show, <p>{fmt.Sprintf("items: %d", len(items))}</p>)}
			{ItemsList(lis)}
		</div>
	)

	return banner
}
