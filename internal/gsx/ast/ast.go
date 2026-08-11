// Package ast defines the GSX syntax tree: the tag expressions embedded in a
// `.gsx` file, plus the Go source fragments spliced between them.
package ast

// Node is a GSX tag-expression node.
//
// Off is the byte offset of the node's first character in the original `.gsx`
// source, so diagnostics can point at the exact character the user typed.
type Node interface {
	node()
	Off() int
}

// Text is literal character data between tags.
type Text struct {
	Value string
	Pos   int
}

func (Text) node()      {}
func (t Text) Off() int { return t.Pos }

// Expr is a `{...}` splice: a Go expression appearing as a child or as an
// attribute value.
//
// Src is Go source. Any tag expressions nested inside it have been replaced by
// zero-argument placeholder calls (`__gsx_sub_1()`), with the corresponding tag
// recorded in Nested under that name. Lowering substitutes them back in, which
// lets a nested tag be lowered with the same type context as its parent rather
// than in isolation.
type Expr struct {
	Src    string
	Nested map[string]Node
	Pos    int
}

func (Expr) node()      {}
func (e Expr) Off() int { return e.Pos }

// AttrKind distinguishes the three attribute forms.
type AttrKind int

const (
	// AttrBool is a valueless attribute: `disabled`.
	AttrBool AttrKind = iota
	// AttrString is a quoted literal: `class="card"`.
	AttrString
	// AttrExpr is a Go splice: `class={s}`. A bare `{expr}` in attribute
	// position — an attribute-node injection — is an AttrExpr with an empty Key.
	AttrExpr
)

// Attr is a single attribute in a start tag.
type Attr struct {
	Key  string
	Kind AttrKind
	// Value is the literal string for AttrString, or Go expression source for
	// AttrExpr. Nested carries that expression's nested tags, as on Expr.
	Value  string
	Nested map[string]Node
	Pos    int
}

// Element is a tag: `<div class="x">...</div>` or `<input />`.
type Element struct {
	Tag         string
	Attrs       []Attr
	Children    []Node
	SelfClosing bool
	Pos         int
}

func (Element) node()      {}
func (e Element) Off() int { return e.Pos }
