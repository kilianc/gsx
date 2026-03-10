package e2e

func init() {
	GSXFunctions["struct_fields"] = func() Node {
		return StructFields(PageData{
			Title: <h1>hello</h1>,
		})
	}
}

type PageData struct {
	Title Node
}

func StructFields(d PageData) Node {
	return <div>{d.Title}</div>
}
