package parse

import (
	"html"
	"strconv"
	"strings"
	"unicode/utf8"
)

// normalizeText applies JSX's whitespace rules to a run of literal character
// data, returning "" when the run carries no content and should be dropped.
//
// The rules, which match Babel's cleanJSXElementLiteralChild:
//
//   - A run containing no newline is passed through untouched, so
//     `<b>a </b><i>b</i>` keeps its meaningful space.
//   - Otherwise every line is trimmed (leading whitespace except on the first
//     line, trailing whitespace except on the last), blank lines are dropped,
//     and the remaining lines are joined with a single space.
//
// That is what makes indented markup render the way it looks:
//
//	<p>
//	  Hello
//	  there
//	</p>
//
// produces "Hello there" rather than "\n  Hello\n  there\n".
func normalizeText(raw string) string {
	if !strings.ContainsAny(raw, "\n\r") {
		return raw
	}

	lines := splitLines(raw)

	lastNonEmpty := -1
	for i, l := range lines {
		if strings.ContainsFunc(l, isNotSpaceOrTab) {
			lastNonEmpty = i
		}
	}
	if lastNonEmpty < 0 {
		// Whitespace-only run spanning a line break: pure layout, no content.
		return ""
	}

	var sb strings.Builder
	for i, line := range lines {
		trimmed := strings.ReplaceAll(line, "\t", " ")
		if i != 0 {
			trimmed = strings.TrimLeft(trimmed, " ")
		}
		if i != len(lines)-1 {
			trimmed = strings.TrimRight(trimmed, " ")
		}
		if trimmed == "" {
			continue
		}
		sb.WriteString(trimmed)
		if i != lastNonEmpty {
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

func isNotSpaceOrTab(r rune) bool { return r != ' ' && r != '\t' }

// splitLines splits on \n, \r\n and lone \r.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// decodeEntities resolves HTML character references in literal text, the way
// JSX does at compile time.
//
// This is necessary because the renderer escapes what it is given: without
// decoding, `&amp;` in a `.gsx` file would be re-escaped to `&amp;amp;` and the
// reader would see the literal text "&amp;".
//
// Only well-formed references — a name or numeric form terminated by `;` — are
// decoded. That avoids the legacy HTML behaviour where a bare `&not` decodes to
// `¬`, which would surprise anyone writing `&notice` or a query string like
// `?a=1&next=2`.
func decodeEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}

	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '&' {
			sb.WriteByte(s[i])
			i++
			continue
		}
		end, ok := entityEnd(s, i)
		if !ok {
			sb.WriteByte(s[i])
			i++
			continue
		}
		sb.WriteString(decodeRef(s[i:end]))
		i = end
	}
	return sb.String()
}

// decodeRef decodes one complete character reference, or returns it unchanged
// if it does not name anything.
func decodeRef(ref string) string {
	// Numeric references are decoded directly: they are unambiguous, and doing
	// them here keeps the named-reference guard below simple.
	if len(ref) > 2 && ref[1] == '#' {
		digits, base := ref[2:len(ref)-1], 10
		if digits[0] == 'x' || digits[0] == 'X' {
			digits, base = digits[1:], 16
		}
		n, err := strconv.ParseInt(digits, base, 32)
		if err != nil || !utf8.ValidRune(rune(n)) || n == 0 {
			return ref
		}
		return string(rune(n))
	}

	dec := html.UnescapeString(ref)
	if dec == ref {
		return ref
	}

	// html.UnescapeString implements HTML's legacy longest-prefix matching, so
	// it decodes the `&not` inside `&notarealentity;` and hands back
	// "¬arealentity;". A full match consumes the trailing semicolon; a partial
	// one always leaves it attached to the rest of the name. `&semi;` is the
	// only named reference whose value is itself a semicolon.
	if len(dec) > 1 && strings.HasSuffix(dec, ";") {
		return ref
	}
	return dec
}

// entityEnd returns the index just past the `;` of the character reference
// starting at i, and whether one was found.
func entityEnd(s string, i int) (int, bool) {
	j := i + 1
	if j < len(s) && s[j] == '#' {
		j++
		if j < len(s) && (s[j] == 'x' || s[j] == 'X') {
			j++
			start := j
			for j < len(s) && isHexDigit(s[j]) {
				j++
			}
			if j == start {
				return 0, false
			}
		} else {
			start := j
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			if j == start {
				return 0, false
			}
		}
	} else {
		start := j
		for j < len(s) && isAlphaNum(s[j]) {
			j++
		}
		if j == start {
			return 0, false
		}
	}
	if j < len(s) && s[j] == ';' {
		return j + 1, true
	}
	return 0, false
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func isAlphaNum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isCommentOnly reports whether a `{...}` splice contains nothing but comments
// and whitespace, i.e. it is a JSX comment: `{/* note */}`.
func isCommentOnly(src string) bool {
	s := &scanner{src: []byte(src)}
	for !s.eof() {
		switch {
		case s.startsWith("//"):
			s.readLineComment()
		case s.startsWith("/*"):
			s.readBlockComment()
		default:
			switch s.peek() {
			case ' ', '\t', '\n', '\r':
				s.next()
			default:
				return false
			}
		}
	}
	return true
}
