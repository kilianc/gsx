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

// prevToken returns the token immediately before the cursor, skipping spaces:
// a whole identifier or number if the preceding byte is a word byte, and
// otherwise that single byte on its own. It returns "" at the start of input.
//
// This is the one place the scanner looks backwards. It exists so tag detection
// can tell an operator from an operand without lexing Go — see atGoTagStart.
func (s *scanner) prevToken() string {
	j := s.i - 1
	for j >= 0 && isSpace(s.src[j]) {
		j--
	}
	if j < 0 {
		return ""
	}
	if !isWordByte(s.src[j]) {
		return string(s.src[j])
	}
	end := j + 1
	for j >= 0 && isWordByte(s.src[j]) {
		j--
	}
	return string(s.src[j+1 : end])
}

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
