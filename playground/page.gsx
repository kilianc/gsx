package playground

import "fmt"

func Page(title string, items []string, show bool) Node {
	if title == "" {
		return <p>no title</p>
	}

	var lis []Node
	for _, it := range items {
		lis = append(lis, <li class="item">{it}</li>)
	}

	banner := (
		<div class="page">
			<h1>{title}</h1>
			{If(show, <p>{fmt.Sprintf("items: %d", len(items))}</p>)}
			<ul>{Group(lis)}</ul>
		</div>
	)

	return banner
}
