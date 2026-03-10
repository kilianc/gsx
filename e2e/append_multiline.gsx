package e2e

func init() {
	GSXFunctions["append_multiline"] = func() Node {
		return AppendMultiline()
	}
}

func AppendMultiline() Node {
	var rows []Node
	rows = append(rows, (
		<tr>
			<td>hello</td>
		</tr>
	))
	rows = append(rows, (
		<tr>
			<td>world</td>
		</tr>
	))
	return <div>{rows}</div>
}
