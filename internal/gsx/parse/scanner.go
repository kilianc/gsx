package parse

import "bytes"

// scanner is a byte cursor over `.gsx` source.
//
// It is deliberately not a Go lexer. Its only job is to walk the file while
// skipping the constructs a `<` could legitimately appear inside — comments and
// the three Go literal forms — so that tag detection never fires on, say, the
// `<` in `a < b` inside a string.
type scanner struct {
	src []byte
	i   int

	// prev is the last significant token passed in Go-code position, recorded as
	// the cursor walks over it. It is what tells `<div>` from `a<b`, since only
	// the latter follows something that ends an operand — see atGoTagStart.
	prev string
}

func (s *scanner) eof() bool { return s.i >= len(s.src) }

func (s *scanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.src[s.i]
}

func (s *scanner) peekN(n int) byte {
	j := s.i + n
	if j >= len(s.src) || j < 0 {
		return 0
	}
	return s.src[j]
}

func (s *scanner) next() byte {
	if s.eof() {
		return 0
	}
	b := s.src[s.i]
	s.i++
	return b
}

func (s *scanner) startsWith(prefix string) bool {
	return bytes.HasPrefix(s.src[s.i:], []byte(prefix))
}

func (s *scanner) readLineComment() string {
	start := s.i
	for !s.eof() && s.next() != '\n' {
	}
	return string(s.src[start:s.i])
}

func (s *scanner) readBlockComment() string {
	start := s.i
	s.next()
	s.next()
	for !s.eof() && !s.startsWith("*/") {
		s.next()
	}
	if s.startsWith("*/") {
		s.next()
		s.next()
	}
	return string(s.src[start:s.i])
}

func (s *scanner) readStringLit() string {
	start := s.i
	s.next()
	for !s.eof() {
		c := s.next()
		if c == '\\' {
			_ = s.next()
			continue
		}
		if c == '"' {
			break
		}
	}
	return string(s.src[start:s.i])
}

func (s *scanner) readRuneLit() string {
	start := s.i
	s.next()
	for !s.eof() {
		c := s.next()
		if c == '\\' {
			_ = s.next()
			continue
		}
		if c == '\'' {
			break
		}
	}
	return string(s.src[start:s.i])
}

func (s *scanner) readRawString() string {
	start := s.i
	s.next()
	for !s.eof() && s.next() != '`' {
	}
	return string(s.src[start:s.i])
}

func (s *scanner) skipSpace() {
	for isSpace(s.peek()) {
		s.next()
	}
}

// markToken records tok as the last significant token in Go-code position.
//
// Whitespace and comments are not significant and do not call it, which is the
// whole advantage of recording forwards: they are already skipped by the time
// the cursor reaches the `<`, so `a /* note */ <b` still sees `a`.
func (s *scanner) markToken(tok string) { s.prev = tok }

// prevToken returns the last recorded token, or "" at the start of a Go region
// — the file, or the `{` of a splice.
func (s *scanner) prevToken() string { return s.prev }

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '_'
}

func isTagStart(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isTagNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '.' || b == '_'
}

func isAttrNameByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b == '-' || b == '_' || b == ':'
}
