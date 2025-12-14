package ast

type Node interface {
	node()
}

type Text struct {
	Value string
}

func (Text) node() {}

type Expr struct {
	Src string
}

func (Expr) node() {}

type AttrKind int

const (
	AttrBool AttrKind = iota
	AttrString
	AttrExpr
)

type Attr struct {
	Key  string
	Kind AttrKind
	// Value is the literal string (for AttrString) or expression source (for AttrExpr).
	Value string
}

type Element struct {
	Tag         string
	Attrs       []Attr
	Children    []Node
	SelfClosing bool
}

func (Element) node() {}
