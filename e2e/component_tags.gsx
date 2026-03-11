package e2e

import "github.com/kilianc/gsx/e2e/helpers"

func init() {
	GSXFunctions["component_tags"] = func() Node {
		return ComponentTags()
	}
}

func Card(children ...Node) Node {
	return <div class="card">{children}</div>
}

func ComponentTags() Node {
	return (
		<div>
			<Card class="primary"><p>hello</p></Card>
			<Card/>
			<helpers.Wrapper><span>wrapped</span></helpers.Wrapper>
		</div>
	)
}
