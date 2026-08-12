package ui

func Splice(name string) Node {
	return <div>{name}</div>
	//= { -> punctuation.section.embedded
}

func Comment() Node {
	return <div>{/* dropped */}</div>
	//= /* dropped */ -> comment
}

func Spread(attrs []Node) Node {
	return <div {...attrs}>x</div>
	//= ... -> keyword.operator.spread
}

func Entity() Node {
	return <p>Tom &amp; Jerry</p>
	//= &amp; -> constant.character.entity
}
