package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// Error is a parse error carrying the exact source position that caused it.
//
// Its message renders like a Go compiler error, followed by the offending line
// and a caret:
//
//	page.gsx:12:22: mismatched closing tag </span>, expected </div>
//
//	  12 |     <div class="x">hello</span>
//	     |                         ^
type Error struct {
	// Path is the source file the offset refers to. It may be empty.
	Path string
	// Off is the byte offset of the error in Src.
	Off int
	Msg string
	// Src is the full source, retained so the snippet can be rendered lazily.
	Src []byte
}

func (e *Error) Error() string {
	line, col := LineCol(e.Src, e.Off)

	var sb strings.Builder
	if e.Path != "" {
		sb.WriteString(e.Path)
		sb.WriteByte(':')
	}
	fmt.Fprintf(&sb, "%d:%d: %s", line, col, e.Msg)

	if snip := snippet(e.Src, line, col); snip != "" {
		sb.WriteString("\n\n")
		sb.WriteString(snip)
	}
	return sb.String()
}

// Position returns the 1-based line and column of the error.
func (e *Error) Position() (line, col int) { return LineCol(e.Src, e.Off) }

// errorf builds an Error at off. Src and Path are filled in by the caller that
// owns them, so parsing helpers only need the offset and message.
func (p *parser) errorf(off int, format string, args ...any) *Error {
	return &Error{
		Path: p.path,
		Off:  off,
		Msg:  fmt.Sprintf(format, args...),
		Src:  p.s.src,
	}
}

// LineCol converts a byte offset into a 1-based line and column.
//
// The column counts bytes, not runes: it is used to place a caret under a
// monospaced rendering of the same bytes, and every character GSX's parser can
// fail on is ASCII.
func LineCol(src []byte, off int) (line, col int) {
	if off < 0 {
		off = 0
	}
	if off > len(src) {
		off = len(src)
	}
	line = 1
	lineStart := 0
	for i := 0; i < off; i++ {
		if src[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return line, off - lineStart + 1
}

// snippet renders the source line plus a caret under the offending column.
func snippet(src []byte, line, col int) string {
	lines := strings.Split(string(src), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	text := strings.TrimRight(lines[line-1], "\r")

	// Tabs would misalign the caret, so render them as single spaces in both
	// the source line and the padding beneath it.
	text = strings.ReplaceAll(text, "\t", " ")

	gutter := strconv.Itoa(line)
	pad := strings.Repeat(" ", len(gutter))

	caretPad := col - 1
	if caretPad < 0 {
		caretPad = 0
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "  %s | %s\n", gutter, text)
	fmt.Fprintf(&sb, "  %s | %s^", pad, strings.Repeat(" ", caretPad))
	return sb.String()
}
