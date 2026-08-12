package ui

// Go operators must never be mistaken for tag punctuation.
func Compare(a int, b int) bool {
	return a < b
	//= !< -> entity.name.tag
	//= !< -> punctuation.definition.tag
}

func Shift(a int) int {
	return a << 2
	//= !<< -> punctuation.definition.tag
}

func Receive(ch chan int) int {
	v := <-ch
	//= !<- -> punctuation.definition.tag
	return v
}

// A plain Go assignment is not an attribute.
func Assign() {
	count = 1
	//= !count -> entity.other.attribute-name
}
