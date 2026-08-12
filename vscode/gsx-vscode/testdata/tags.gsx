package ui

func Page(name string, ok bool) Node {
	return <div class="card" id={id}>hello</div>
	//= <div -> entity.name.tag
	//= class -> entity.other.attribute-name
	//= "card" -> string
	//= </div -> entity.name.tag

	x := <input required disabled={locked} />
	//= <input -> entity.name.tag
	//= required -> entity.other.attribute-name
	//= disabled -> entity.other.attribute-name
}

func Component() Node {
	return <Card variant="primary"><p>hi</p></Card>
	//= <Card -> support.class.component
	//= <p -> entity.name.tag
}

func Dotted() Node {
	return <ui.widgets.Card>body</ui.widgets.Card>
	//= <ui.widgets.Card -> support.class.component
}

func Fragment() Node {
	return <><p>a</p><p>b</p></>
	//= <> -> punctuation.definition.tag
}
