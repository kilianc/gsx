package ui

func Cart(items []Item) Node {
	var rows []Node
	for _, it := range items {
		class := "row"
		if it.Sale {
			class += " sale"
		}
		rows = append(rows, <li class={class}>{it.Name}</li>)
	}
	return <ul class="cart">{rows}</ul>
}
