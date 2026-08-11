// Package parse turns `.gsx` source into Go source plus a set of GSX tag trees.
//
// A `.gsx` file is a Go file with one extra expression form: the tag expression.
// Rather than teach Go's parser about tags, RewriteTags replaces each top-level
// tag expression with a call to a placeholder function — `__gsx_expr_1()` — so
// the result is valid Go that go/parser accepts. The compiler lowers each tag
// separately and substitutes the lowered expression back in at the placeholder.
package parse

import (
	"fmt"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/ast"
)

// Tag is one top-level tag expression found in a `.gsx` file.
type Tag struct {
	// Name is the placeholder function name that stands in for this tag in the
	// rewritten Go source, e.g. "__gsx_expr_1".
	Name string
	Node ast.Node

	// SrcStart and SrcEnd bound the tag in the original `.gsx` source.
	SrcStart, SrcEnd int
	// TgtStart and TgtEnd bound the `__gsx_expr_N()` call in the rewritten source.
	TgtStart, TgtEnd int
}

type parser struct {
	s    *scanner
	path string
	// subN numbers nested placeholders file-wide, so a name is unique no matter
	// how deeply tags and `{...}` splices interleave.
	subN int
}

// RewriteTags replaces every top-level tag expression in src with a placeholder
// call, returning the rewritten Go source and the tags that were extracted.
func RewriteTags(path string, src []byte) ([]byte, []Tag, error) {
	p := &parser{s: &scanner{src: src}, path: path}

	var out strings.Builder
	var tags []Tag

	for !p.s.eof() {
		if p.skipNonCode(&out) {
			continue
		}
		if p.atTagStart() {
			srcStart := p.s.i
			node, err := p.parseTag()
			if err != nil {
				return nil, nil, err
			}
			name := fmt.Sprintf("__gsx_expr_%d", len(tags)+1)

			tgtStart := out.Len()
			out.WriteString(name)
			out.WriteString("()")

			tags = append(tags, Tag{
				Name:     name,
				Node:     node,
				SrcStart: srcStart,
				SrcEnd:   p.s.i,
				TgtStart: tgtStart,
				TgtEnd:   out.Len(),
			})
			continue
		}
		out.WriteByte(p.s.next())
	}

	return []byte(out.String()), tags, nil
}

// skipNonCode copies a comment or Go literal through verbatim, so that a `<`
// inside one is never mistaken for a tag. It reports whether it consumed input.
func (p *parser) skipNonCode(out *strings.Builder) bool {
	switch {
	case p.s.startsWith("//"):
		out.WriteString(p.s.readLineComment())
	case p.s.startsWith("/*"):
		out.WriteString(p.s.readBlockComment())
	default:
		switch p.s.peek() {
		case '"':
			out.WriteString(p.s.readStringLit())
		case '\'':
			out.WriteString(p.s.readRuneLit())
		case '`':
			out.WriteString(p.s.readRawString())
		default:
			return false
		}
	}
	return true
}

// atTagStart reports whether the cursor sits on the `<` of a tag expression.
//
// A tag is `<name` or the fragment opener `<>`. Neither `<>` nor `<` followed
// by a letter can begin a valid Go expression at a point where a tag is
// allowed, so this never misfires on comparison or receive operators.
func (p *parser) atTagStart() bool {
	if p.s.peek() != '<' {
		return false
	}
	b := p.s.peekN(1)
	return isTagStart(b) || b == '>'
}

// parseTag parses a single element or fragment starting at '<'.
func (p *parser) parseTag() (ast.Node, error) {
	start := p.s.i
	p.s.next() // '<'

	// Fragment: `<>...</>`, which groups siblings without emitting an element.
	if p.s.peek() == '>' {
		p.s.next()
		children, err := p.parseChildren("", start)
		if err != nil {
			return nil, err
		}
		return ast.Fragment{Children: children, Pos: start}, nil
	}

	nameStart := p.s.i
	for isTagNameByte(p.s.peek()) {
		p.s.next()
	}
	tag := string(p.s.src[nameStart:p.s.i])
	if tag == "" {
		return nil, p.errorf(start, "expected a tag name after `<`")
	}

	attrs, selfClosing, err := p.parseAttrs(tag, start)
	if err != nil {
		return nil, err
	}
	if selfClosing {
		return ast.Element{Tag: tag, Attrs: attrs, SelfClosing: true, Pos: start}, nil
	}

	children, err := p.parseChildren(tag, start)
	if err != nil {
		return nil, err
	}
	return ast.Element{Tag: tag, Attrs: attrs, Children: children, Pos: start}, nil
}

// parseAttrs consumes everything from after the tag name through `>` or `/>`.
func (p *parser) parseAttrs(tag string, tagPos int) (attrs []ast.Attr, selfClosing bool, err error) {
	for {
		p.s.skipSpace()

		if p.s.eof() {
			return nil, false, p.errorf(tagPos, "unclosed `<%s`: reached end of file while reading attributes", tag)
		}
		if p.s.startsWith("/>") {
			p.s.next()
			p.s.next()
			return attrs, true, nil
		}
		if p.s.peek() == '>' {
			p.s.next()
			return attrs, false, nil
		}

		// A bare `{expr}` in attribute position injects an attribute node.
		if p.s.peek() == '{' {
			pos := p.s.i
			src, nested, err := p.readBracedExpr()
			if err != nil {
				return nil, false, err
			}
			if isCommentOnly(src) {
				continue
			}
			attrs = append(attrs, ast.Attr{
				Kind:   ast.AttrExpr,
				Value:  strings.TrimSpace(src),
				Nested: nested,
				Pos:    pos,
			})
			continue
		}

		attr, err := p.parseAttr(tag)
		if err != nil {
			return nil, false, err
		}
		attrs = append(attrs, attr)
	}
}

func (p *parser) parseAttr(tag string) (ast.Attr, error) {
	pos := p.s.i
	for isAttrNameByte(p.s.peek()) {
		p.s.next()
	}
	key := string(p.s.src[pos:p.s.i])
	if key == "" {
		return ast.Attr{}, p.errorf(pos, "unexpected %q in <%s>: expected an attribute name, `>` or `/>`", string(p.s.peek()), tag)
	}
	p.s.skipSpace()

	if p.s.peek() == '?' && p.s.peekN(1) == '=' {
		return ast.Attr{}, p.errorf(p.s.i, "unsupported `?=` syntax for attribute %q; use {If(cond, Attr(...))} instead", key)
	}

	if p.s.peek() != '=' {
		return ast.Attr{Key: key, Kind: ast.AttrBool, Pos: pos}, nil
	}
	p.s.next() // '='
	p.s.skipSpace()

	switch p.s.peek() {
	case '"':
		p.s.next()
		valStart := p.s.i
		for !p.s.eof() && p.s.peek() != '"' {
			p.s.next()
		}
		if p.s.eof() {
			return ast.Attr{}, p.errorf(valStart-1, "unterminated string value for attribute %q", key)
		}
		val := string(p.s.src[valStart:p.s.i])
		p.s.next() // closing quote
		return ast.Attr{Key: key, Kind: ast.AttrString, Value: val, Pos: pos}, nil

	case '{':
		src, nested, err := p.readBracedExpr()
		if err != nil {
			return ast.Attr{}, err
		}
		return ast.Attr{
			Key:    key,
			Kind:   ast.AttrExpr,
			Value:  strings.TrimSpace(src),
			Nested: nested,
			Pos:    pos,
		}, nil

	default:
		return ast.Attr{}, p.errorf(p.s.i, "expected `\"` or `{` after `%s=` in <%s>", key, tag)
	}
}

// parseChildren consumes everything up to and including the matching close tag.
// An empty tag means the caller is parsing a fragment, closed by `</>`.
func (p *parser) parseChildren(tag string, tagPos int) ([]ast.Node, error) {
	var kids []ast.Node

	for {
		if p.s.eof() {
			return nil, p.errorf(tagPos, "unclosed %s: reached end of file without a matching %s", openName(tag), closeName(tag))
		}

		if p.s.startsWith("</") {
			closePos := p.s.i
			p.s.next()
			p.s.next()
			nameStart := p.s.i
			for isTagNameByte(p.s.peek()) {
				p.s.next()
			}
			closeTag := string(p.s.src[nameStart:p.s.i])
			p.s.skipSpace()
			if p.s.peek() != '>' {
				return nil, p.errorf(p.s.i, "expected `>` to close %s", closeName(closeTag))
			}
			p.s.next()
			if closeTag != tag {
				return nil, p.errorf(closePos, "mismatched closing tag %s, expected %s", closeName(closeTag), closeName(tag))
			}
			return kids, nil
		}

		if p.atTagStart() {
			n, err := p.parseTag()
			if err != nil {
				return nil, err
			}
			kids = append(kids, n)
			continue
		}

		if p.s.peek() == '{' {
			pos := p.s.i
			src, nested, err := p.readBracedExpr()
			if err != nil {
				return nil, err
			}
			// `{/* note */}` is a JSX comment, not an expression: drop it.
			if isCommentOnly(src) {
				continue
			}
			kids = append(kids, ast.Expr{Src: strings.TrimSpace(src), Nested: nested, Pos: pos})
			continue
		}

		pos := p.s.i
		for !p.s.eof() && p.s.peek() != '<' && p.s.peek() != '{' {
			p.s.next()
		}
		if v := normalizeText(string(p.s.src[pos:p.s.i])); v != "" {
			kids = append(kids, ast.Text{Value: decodeEntities(v), Pos: pos})
		}
	}
}

// openName and closeName render a tag name for a diagnostic, spelling the
// fragment forms as `<>` and `</>` rather than as an empty name.
func openName(tag string) string {
	if tag == "" {
		return "<>"
	}
	return "<" + tag + ">"
}

func closeName(tag string) string {
	if tag == "" {
		return "</>"
	}
	return "</" + tag + ">"
}

// readBracedExpr consumes a `{...}` splice and returns its Go source.
//
// Tag expressions inside the splice are parsed here and replaced by placeholder
// calls, so their offsets stay relative to the original file and — crucially —
// they are lowered later with the same type context as the enclosing tag rather
// than in isolation.
func (p *parser) readBracedExpr() (string, map[string]ast.Node, error) {
	open := p.s.i
	p.s.next() // '{'

	var out strings.Builder
	var nested map[string]ast.Node
	depth := 1

	for !p.s.eof() {
		if p.skipNonCode(&out) {
			continue
		}
		if p.atTagStart() {
			node, err := p.parseTag()
			if err != nil {
				return "", nil, err
			}
			p.subN++
			name := fmt.Sprintf("__gsx_sub_%d", p.subN)
			if nested == nil {
				nested = map[string]ast.Node{}
			}
			nested[name] = node
			out.WriteString(name)
			out.WriteString("()")
			continue
		}

		switch p.s.peek() {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				p.s.next()
				return out.String(), nested, nil
			}
		}
		out.WriteByte(p.s.next())
	}

	return "", nil, p.errorf(open, "unterminated `{`: reached end of file without a matching `}`")
}
