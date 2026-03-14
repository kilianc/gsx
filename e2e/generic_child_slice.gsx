package e2e

func init() {
	GSXFunctions["generic_child_slice"] = func() Node {
		return GenericChildSlice()
	}
}

func Row(children ...Node) Node {
	return <div class="row">{children}</div>
}

func dataRows[T any, C ~string](items []T, cols []C, cellFn func(C, T) Node) []Node {
	rows := make([]Node, 0, len(items))
	for _, item := range items {
		cells := make([]Node, 0, len(cols))
		for _, col := range cols {
			cells = append(cells, cellFn(col, item))
		}
		rows = append(rows, <Row>{cells}</Row>)
	}
	return rows
}

func GenericChildSlice() Node {
	items := []map[string]string{
		{"name": "alice", "role": "admin"},
		{"name": "bob", "role": "user"},
	}
	cols := []string{"name", "role"}
	rows := dataRows(items, cols, func(col string, item map[string]string) Node {
		return <span>{item[col]}</span>
	})
	return Row(rows...)
}
