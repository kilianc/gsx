// Package format implements `gsx fmt`.
//
// The style is deliberately conservative. Go code is formatted exactly as
// gofmt would format it; tag expressions keep the shape their author gave them
// and are only re-indented to match their new surroundings.
//
// Reflowing markup — deciding when attributes wrap, when children move onto
// their own lines — is where formatters become contentious, and a formatter
// that rewrites markup on every save churns diffs and starts arguments. Doing
// less here means `gsx fmt` can be turned on for a whole team without anyone
// having to agree about markup style first, and it is the part gofmt cannot do
// for them anyway.
package format

import (
	goformat "go/format"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/parse"
)

// Source formats GSX source.
func Source(path string, src []byte) ([]byte, error) {
	_, tags, err := parse.RewriteTags(path, src)
	if err != nil {
		return nil, err
	}

	spans := absorbParens(string(src), tags)

	// With every tag replaced by a call, what is left is ordinary Go, so gofmt
	// decides the layout of everything except the markup itself.
	gofmted, err := goformat.Source([]byte(blankOut(string(src), spans)))
	if err != nil {
		return nil, err
	}

	out, err := restoreTags(string(gofmted), string(src), spans)
	if err != nil {
		return nil, err
	}
	return []byte(out), nil
}

// span is one tag expression, plus the parentheses wrapping it when the author
// used the multi-line form.
type span struct {
	name       string
	start, end int // byte range in the original source, parentheses included
	tagStart   int // byte offset of the tag itself
	tagEnd     int
	parenized  bool
}

// absorbParens widens each tag to include a surrounding `(` ... `)` when the
// tag sits alone between them.
//
// That form exists because Go inserts a semicolon at the end of a line, so
// `return` followed by markup on the next line does not parse without it. The
// parentheses therefore have to survive formatting, and the simplest way to
// guarantee that is to treat them as part of the thing being preserved.
func absorbParens(src string, tags []parse.Tag) []span {
	out := make([]span, 0, len(tags))
	for _, t := range tags {
		s := span{name: t.Name, start: t.SrcStart, end: t.SrcEnd, tagStart: t.SrcStart, tagEnd: t.SrcEnd}

		if open, ok := parenBefore(src, t.SrcStart); ok {
			if close, ok := parenAfter(src, t.SrcEnd); ok {
				s.start, s.end, s.parenized = open, close+1, true
			}
		}
		out = append(out, s)
	}
	return out
}

// parenBefore reports the offset of a `(` that precedes off across a newline.
func parenBefore(src string, off int) (int, bool) {
	i := off - 1
	sawNewline := false
	for i >= 0 {
		switch src[i] {
		case ' ', '\t', '\r':
		case '\n':
			sawNewline = true
		case '(':
			return i, sawNewline
		default:
			return 0, false
		}
		i--
	}
	return 0, false
}

// parenAfter reports the offset of a `)` that follows off across a newline.
func parenAfter(src string, off int) (int, bool) {
	i := off
	sawNewline := false
	for i < len(src) {
		switch src[i] {
		case ' ', '\t', '\r':
		case '\n':
			sawNewline = true
		case ')':
			return i, sawNewline
		default:
			return 0, false
		}
		i++
	}
	return 0, false
}

// blankOut replaces each span with its placeholder call, producing Go that
// gofmt accepts.
func blankOut(src string, spans []span) string {
	var b strings.Builder
	b.Grow(len(src))
	prev := 0
	for _, s := range spans {
		b.WriteString(src[prev:s.start])
		b.WriteString(s.name)
		b.WriteString("()")
		prev = s.end
	}
	b.WriteString(src[prev:])
	return b.String()
}

// restoreTags puts each original tag back where gofmt left its placeholder,
// re-indented from the indentation it had to the indentation it now needs.
func restoreTags(gofmted, src string, spans []span) (string, error) {
	var b strings.Builder
	b.Grow(len(gofmted))

	rest := gofmted
	for _, s := range spans {
		needle := s.name + "()"
		i := strings.Index(rest, needle)
		if i < 0 {
			// gofmt does not delete code, so this cannot happen for a tag that
			// was in the input; treat it as a bug rather than silently dropping
			// the user's markup.
			return "", &parse.Error{
				Off: s.tagStart,
				Msg: "internal: lost placeholder " + s.name + " while formatting",
				Src: []byte(src),
			}
		}

		b.WriteString(rest[:i])

		at := lineIndentBefore(rest, i)
		tag := src[s.tagStart:s.tagEnd]

		if s.parenized {
			// Re-emit the multi-line form, with the markup one level in.
			inner := at + "\t"
			b.WriteString("(\n")
			b.WriteString(inner)
			b.WriteString(reindent(tag, lineIndentBefore(src, s.tagStart), inner))
			b.WriteString("\n")
			b.WriteString(at)
			b.WriteString(")")
		} else {
			b.WriteString(reindent(tag, lineIndentBefore(src, s.tagStart), at))
		}

		rest = rest[i+len(needle):]
	}
	b.WriteString(rest)
	return b.String(), nil
}

// lineIndentBefore returns the leading whitespace of the line containing off.
func lineIndentBefore(s string, off int) string {
	start := strings.LastIndexByte(s[:off], '\n') + 1
	line := s[start:off]
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// reindent shifts every line of a tag after the first from oldIndent to
// newIndent, leaving the relative shape of the markup untouched.
func reindent(tagSrc, oldIndent, newIndent string) string {
	if !strings.Contains(tagSrc, "\n") || oldIndent == newIndent {
		return tagSrc
	}

	lines := strings.Split(tagSrc, "\n")
	for i := 1; i < len(lines); i++ {
		// A line indented at least as far as the tag's own indentation is part
		// of the markup's structure and moves with it. A line indented less —
		// which a hand-formatted tag can contain — is left alone rather than
		// guessed at.
		if rest, ok := strings.CutPrefix(lines[i], oldIndent); ok {
			lines[i] = newIndent + rest
		}
	}
	return strings.Join(lines, "\n")
}
