// Package highlight tokenizes GSX source for display.
//
// Off-the-shelf Go highlighters do not know about tag expressions, and HTML
// highlighters do not know about Go, so a `.gsx` snippet comes out wrong in
// either. This tokenizer understands both and the boundary between them, which
// is the part that actually needs explaining on a documentation page.
package highlight

import (
	"html"
	"strings"
)

// Class is the token category, used as a CSS class name.
type Class string

const (
	ClassNone    Class = ""
	ClassComment Class = "c"  // comments
	ClassString  Class = "s"  // string, rune and raw literals
	ClassNumber  Class = "n"  // numeric literals
	ClassKeyword Class = "k"  // Go keywords
	ClassType    Class = "t"  // builtin type and constant names
	ClassTag     Class = "tg" // tag names, including < > /
	ClassAttr    Class = "at" // attribute names
	ClassBrace   Class = "br" // the { } delimiting a splice
	ClassPunct   Class = "p"  // tag punctuation such as = and "
)

// Token is a run of source with one class.
type Token struct {
	Text  string
	Class Class
}

// HTML renders src as HTML, wrapping each classified token in a span.
//
// The output is escaped and safe to embed directly in a <pre>.
func HTML(src string) string {
	var sb strings.Builder
	for _, t := range Tokens(src) {
		if t.Class == ClassNone {
			sb.WriteString(html.EscapeString(t.Text))
			continue
		}
		sb.WriteString(`<span class="hl-`)
		sb.WriteString(string(t.Class))
		sb.WriteString(`">`)
		sb.WriteString(html.EscapeString(t.Text))
		sb.WriteString(`</span>`)
	}
	return sb.String()
}

// Tokens splits src into classified runs. Every byte of src appears in exactly
// one token, so concatenating Token.Text reproduces the input.
func Tokens(src string) []Token {
	l := &lexer{src: src}
	l.lexGo()
	return l.merge()
}

type lexer struct {
	src  string
	i    int
	toks []Token

	// prev is the last significant token lexed in Go position, recorded as the
	// cursor walks over it — see atGoTagStart.
	prev string
}

// markToken records tok as the last significant token in Go position. Spaces
// and comments are not tokens and do not call it.
func (l *lexer) markToken(tok string) { l.prev = tok }

// prevToken returns the last recorded token, or "" at the start of a Go region
// — the snippet, or the `{` of a splice.
func (l *lexer) prevToken() string { return l.prev }

func (l *lexer) emit(n int, c Class) {
	if n <= 0 {
		return
	}
	end := min(l.i+n, len(l.src))
	l.toks = append(l.toks, Token{Text: l.src[l.i:end], Class: c})
	l.i = end
}

// emitted is emit for a token whose text the caller still needs, since emit
// advances past it.
func (l *lexer) emitted(n int, c Class) string {
	text := l.src[l.i:min(l.i+n, len(l.src))]
	l.emit(n, c)
	return text
}

func (l *lexer) peek(n int) byte {
	if l.i+n >= len(l.src) {
		return 0
	}
	return l.src[l.i+n]
}

func (l *lexer) has(prefix string) bool {
	return strings.HasPrefix(l.src[l.i:], prefix)
}

// lexGo scans Go code, handing off to lexTag when a tag expression begins.
func (l *lexer) lexGo() {
	for l.i < len(l.src) {
		l.lexGoToken()
	}
}

// lexGoToken lexes one token of Go code and records it as the previous token
// where it is significant — a comment and a space are not.
//
// lexSplice shares it: a splice is Go code too, and the only thing it lexes
// differently is its own braces.
func (l *lexer) lexGoToken() {
	switch {
	case l.has("//"):
		l.emit(l.until("\n"), ClassComment)
	case l.has("/*"):
		l.emit(l.untilAfter("*/", 2), ClassComment)
	case l.peek(0) == '"':
		l.emit(l.quoted('"'), ClassString)
		l.markToken(`"`)
	case l.peek(0) == '\'':
		l.emit(l.quoted('\''), ClassString)
		l.markToken("'")
	case l.peek(0) == '`':
		l.emit(l.untilAfter("`", 1), ClassString)
		l.markToken("`")
	case l.atGoTagStart():
		l.lexTag()
		l.markToken(")") // a tag is a value, like the call it compiles to
	case isDigit(l.peek(0)) && !isNameByte(l.prevByte()):
		l.markToken(l.emitted(l.span(isNumByte), ClassNumber))
	case isNameStart(l.peek(0)):
		n := l.span(isNameByte)
		l.markToken(l.emitted(n, classifyWord(l.src[l.i:l.i+n])))
	case l.peek(0) == '<':
		// Emitted whole, or the second `<` of `a<<b` would be read as the start
		// of a `<b>` tag.
		n := 1
		for l.peek(n) == '<' {
			n++
		}
		if c := l.peek(n); c == '=' || c == '-' {
			n++
		}
		l.emit(n, ClassNone)
		l.markToken("<")
	default:
		b := l.peek(0)
		l.emit(1, ClassNone)
		if !isSpace(b) {
			l.markToken(string(b))
		}
	}
}

// atTagStart reports whether the cursor sits on the `<` of a tag in child
// position, where the surrounding text is markup and every `<` opens a tag or
// a close tag. In Go code use atGoTagStart.
func (l *lexer) atTagStart() bool {
	return l.peek(0) == '<' && (isNameStart(l.peek(1)) || l.peek(1) == '>')
}

// atGoTagStart is atTagStart for Go code — the top level of a snippet, or the
// inside of a `{...}` splice — where `<` is more often an operator than a tag.
//
// It ports parse.atGoTagStart, down to recording the previous token forwards
// rather than scanning back for it, so that a comment between the operand and
// the operator does not hide the operand here either. Divergence is what this
// is written to avoid: colouring `a<b` as markup where the compiler reads a
// comparison teaches the syntax wrong, and colouring a real tag as Go leaves
// the one construct the page exists to explain unhighlighted.
func (l *lexer) atGoTagStart() bool {
	return l.atTagStart() && !endsOperand(l.prevToken())
}

// endsOperand reports whether tok closes an operand, making a `<` right after
// it a comparison or a shift rather than a tag. It mirrors parse.endsOperand.
func endsOperand(tok string) bool {
	if tok == "" {
		return false // start of input: nothing to compare against
	}
	if isNameByte(tok[0]) {
		// A keyword is punctuation, not a value: `return <div>` is a tag, while
		// `count<max` is a comparison.
		return !goExprKeywords[tok]
	}
	switch tok {
	case ")", "]", "}", `"`, "'", "`":
		return true
	}
	// Everything else is punctuation or an operator, and an operator is the one
	// thing a tag is guaranteed to be able to follow: `ch <- <div/>` sends a tag.
	return false
}

// goExprKeywords are the Go keywords an expression can directly follow. Every
// other word is an operand — a variable, a field, a number.
var goExprKeywords = map[string]bool{
	"return": true,
	"case":   true,
	"else":   true,
	"if":     true,
	"for":    true,
	"switch": true,
	"range":  true,
	"go":     true,
	"defer":  true,
}

// lexTag scans a tag expression: the name, its attributes, and its children,
// recursing for nested tags and returning to Go inside `{...}` splices.
func (l *lexer) lexTag() {
	l.emit(1, ClassTag) // '<'

	if l.peek(0) == '>' { // fragment
		l.emit(1, ClassTag)
		l.lexChildren()
		return
	}

	l.emit(l.span(isTagNameByte), ClassTag)

	// Attributes.
	for l.i < len(l.src) {
		switch {
		case l.has("/>"):
			l.emit(2, ClassTag)
			return
		case l.peek(0) == '>':
			l.emit(1, ClassTag)
			l.lexChildren()
			return
		case isSpace(l.peek(0)):
			l.emit(l.span(isSpace), ClassNone)
		case l.peek(0) == '{':
			l.lexSplice()
		case l.peek(0) == '=':
			l.emit(1, ClassPunct)
		case l.peek(0) == '"':
			l.emit(l.quoted('"'), ClassString)
		case isNameStart(l.peek(0)):
			l.emit(l.span(isAttrNameByte), ClassAttr)
		default:
			l.emit(1, ClassNone)
		}
	}
}

func (l *lexer) lexChildren() {
	for l.i < len(l.src) {
		switch {
		case l.has("</"):
			l.emit(2, ClassTag)
			l.emit(l.span(isTagNameByte), ClassTag)
			l.emit(l.span(isSpace), ClassNone)
			if l.peek(0) == '>' {
				l.emit(1, ClassTag)
			}
			return
		case l.atTagStart():
			l.lexTag()
		case l.peek(0) == '{':
			l.lexSplice()
		default:
			// Literal text up to the next tag or splice.
			n := 0
			for l.i+n < len(l.src) && l.src[l.i+n] != '<' && l.src[l.i+n] != '{' {
				n++
			}
			l.emit(max(n, 1), ClassNone)
		}
	}
}

// lexSplice scans `{...}`, highlighting the braces and lexing the contents as
// Go — which is what makes a nested tag inside a splice highlight correctly.
func (l *lexer) lexSplice() {
	l.emit(1, ClassBrace) // '{'
	l.markToken("{")      // a splice opens a fresh Go region

	depth := 1
	for l.i < len(l.src) {
		// Only a brace at the cursor counts: one inside a comment or a literal
		// is consumed whole by lexGoToken and never reaches this switch.
		switch l.peek(0) {
		case '{':
			depth++
			l.emit(1, ClassBrace)
			l.markToken("{")
			continue
		case '}':
			depth--
			l.emit(1, ClassBrace)
			l.markToken("}")
			if depth == 0 {
				return
			}
			continue
		}
		l.lexGoToken()
	}
}

// merge coalesces adjacent tokens of the same class, so the rendered HTML is
// not one span per character.
func (l *lexer) merge() []Token {
	var out []Token
	for _, t := range l.toks {
		if n := len(out); n > 0 && out[n-1].Class == t.Class {
			out[n-1].Text += t.Text
			continue
		}
		out = append(out, t)
	}
	return out
}

func (l *lexer) prevByte() byte {
	if l.i == 0 {
		return 0
	}
	return l.src[l.i-1]
}

// span returns the length of the run at the cursor satisfying pred, which may
// be zero. Callers that need to guarantee progress must handle that; forcing a
// minimum of one here would let an empty span consume the byte after it.
func (l *lexer) span(pred func(byte) bool) int {
	n := 0
	for l.i+n < len(l.src) && pred(l.src[l.i+n]) {
		n++
	}
	return n
}

// until returns the length up to (not including) sep, or to end of input.
func (l *lexer) until(sep string) int {
	if j := strings.Index(l.src[l.i:], sep); j >= 0 {
		return j
	}
	return len(l.src) - l.i
}

// untilAfter returns the length through the end of sep, searching from skip
// bytes in so an opening delimiter is not matched as its own close.
func (l *lexer) untilAfter(sep string, skip int) int {
	start := l.i + skip
	if start >= len(l.src) {
		return len(l.src) - l.i
	}
	if j := strings.Index(l.src[start:], sep); j >= 0 {
		return skip + j + len(sep)
	}
	return len(l.src) - l.i
}

// quoted returns the length of a literal delimited by q, honouring backslash
// escapes.
func (l *lexer) quoted(q byte) int {
	n := 1
	for l.i+n < len(l.src) {
		c := l.src[l.i+n]
		if c == '\\' {
			n += 2
			continue
		}
		n++
		if c == q {
			break
		}
		if c == '\n' {
			break
		}
	}
	return min(n, len(l.src)-l.i)
}

var keywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
}

var types = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true, "error": true,
	"float32": true, "float64": true, "int": true, "int8": true, "int16": true,
	"int32": true, "int64": true, "rune": true, "string": true, "uint": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true, "uintptr": true,
	"any": true, "true": true, "false": true, "nil": true, "iota": true,
	// The gomponents vocabulary GSX generates against reads as builtin here.
	"Node": true, "Group": true,
}

func classifyWord(w string) Class {
	switch {
	case keywords[w]:
		return ClassKeyword
	case types[w]:
		return ClassType
	}
	return ClassNone
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
func isNumByte(b byte) bool {
	return isDigit(b) || b == '.' || b == 'x' || b == 'e' || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
func isNameStart(b byte) bool   { return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isNameByte(b byte) bool    { return isNameStart(b) || isDigit(b) }
func isTagNameByte(b byte) bool { return isNameByte(b) || b == '-' || b == '.' }
func isAttrNameByte(b byte) bool {
	return isNameByte(b) || b == '-' || b == ':'
}
