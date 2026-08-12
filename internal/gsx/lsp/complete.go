package lsp

import (
	"regexp"
	"sort"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/gomponents"
)

// completionKind is what the cursor is positioned to complete.
type completionKind int

const (
	// completeNone means the cursor is in ordinary Go, where gopls answers.
	completeNone completionKind = iota
	// completeTag is just after `<`, or partway through a tag name.
	completeTag
	// completeAttribute is inside a start tag, between attributes.
	completeAttribute
	// completeCloseTag is just after `</`.
	completeCloseTag
)

// completionContext describes what to offer at a position.
type completionContext struct {
	kind completionKind
	// prefix is what the user has typed of the name so far.
	prefix string
	// tag is the innermost unclosed tag name, for completeCloseTag.
	tag string
}

// completionAt inspects the text before the cursor and decides what, if
// anything, GSX should complete.
//
// This is a scan rather than a parse because completion runs on a buffer that
// is mid-edit: `<di` is not a valid tag, which is exactly when the user wants
// the suggestion.
func completionAt(src string, offset int) completionContext {
	if offset < 0 || offset > len(src) {
		return completionContext{}
	}
	before := src[:offset]

	// Everything is decided relative to the last `<`. Counting braces from the
	// start of the file would be wrong — Go blocks use braces too, so the
	// function body's own `{` would look like an open splice.
	open := strings.LastIndexByte(before, '<')
	if open < 0 {
		return completionContext{}
	}
	rest := before[open+1:]

	// A `<` only starts a tag when a name, a slash or a `>` follows it. Without
	// this, the `<` in `a < b` is read as a tag and its right-hand side as an
	// attribute name.
	if rest != "" && !strings.HasPrefix(rest, "/") && !isNameStart(rest[0]) {
		return completionContext{}
	}

	// `</` — offer the tag that is still open.
	if strings.HasPrefix(rest, "/") {
		name := rest[1:]
		if !isNamePrefix(name) {
			return completionContext{}
		}
		return completionContext{kind: completeCloseTag, prefix: name, tag: openTagAt(src, open)}
	}

	// A `>` means that start tag is finished, so the cursor is in children:
	// text or a splice, neither of which GSX completes.
	if strings.ContainsRune(rest, '>') {
		return completionContext{}
	}

	// An unclosed `{` inside the start tag is an attribute splice — Go again.
	if unclosedBrace(rest) {
		return completionContext{}
	}

	// Still typing the name: nothing but name characters so far.
	if isNamePrefix(rest) {
		return completionContext{kind: completeTag, prefix: rest}
	}

	// Past the name and inside the start tag, unless inside a quoted value.
	if strings.ContainsAny(rest, " \t\n") && !inQuotes(rest) {
		word := rest[strings.LastIndexAny(rest, " \t\n")+1:]
		if isAttrPrefix(word) {
			return completionContext{kind: completeAttribute, prefix: word}
		}
	}
	return completionContext{}
}

// unclosedBrace reports whether s ends inside an unclosed `{`.
func unclosedBrace(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}

// inQuotes reports whether the text ends inside a double-quoted value.
func inQuotes(s string) bool { return strings.Count(s, `"`)%2 == 1 }

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNamePrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c == '.' || c == '-' || c == '_' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func isAttrPrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c == '-' || c == '_' || c == ':' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// openTagAt returns the name of the innermost tag still open at offset, so a
// `</` can be completed with the right name.
func openTagAt(src string, offset int) string {
	var stack []string

	for i := 0; i < offset && i < len(src); i++ {
		if src[i] != '<' {
			continue
		}
		if i+1 < len(src) && src[i+1] == '/' {
			// Closing tag: pop.
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		name := nameAt(src, i+1)
		if name == "" {
			continue
		}
		// Self-closing tags never enter the stack.
		if end := strings.IndexByte(src[i:], '>'); end > 0 && src[i+end-1] == '/' {
			continue
		}
		stack = append(stack, name)
	}

	if len(stack) == 0 {
		return ""
	}
	return stack[len(stack)-1]
}

func nameAt(src string, i int) string {
	start := i
	for i < len(src) {
		c := src[i]
		if c == '.' || c == '-' || c == '_' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			i++
			continue
		}
		break
	}
	return src[start:i]
}

// completionItem is one suggestion.
type completionItem struct {
	Label      string `json:"label"`
	Kind       int    `json:"kind"`
	Detail     string `json:"detail,omitempty"`
	InsertText string `json:"insertText,omitempty"`
	// SortText keeps the more relevant group above the alphabetical noise.
	SortText string `json:"sortText,omitempty"`
}

// LSP CompletionItemKind values.
const (
	kindClass    = 7
	kindProperty = 10
	kindKeyword  = 14
)

// completionsFor builds the items for a context.
func completionsFor(ctx completionContext, src string) []completionItem {
	switch ctx.kind {
	case completeTag:
		return tagCompletions(src)
	case completeAttribute:
		return attributeCompletions()
	case completeCloseTag:
		if ctx.tag == "" {
			return nil
		}
		return []completionItem{{
			Label:  ctx.tag + ">",
			Kind:   kindClass,
			Detail: "close <" + ctx.tag + ">",
		}}
	}
	return nil
}

func tagCompletions(src string) []completionItem {
	var out []completionItem

	// Components defined in this file come first: they are the ones the author
	// is most likely reaching for, and they are not discoverable anywhere else.
	for _, name := range componentNames(src) {
		out = append(out, completionItem{
			Label:    name,
			Kind:     kindClass,
			Detail:   "component",
			SortText: "0" + name,
		})
	}

	for _, tag := range gomponents.ElementNames() {
		out = append(out, completionItem{
			Label:    tag,
			Kind:     kindKeyword,
			Detail:   "HTML element",
			SortText: "1" + tag,
		})
	}
	return out
}

func attributeCompletions() []completionItem {
	names := gomponents.AttributeNames()
	out := make([]completionItem, 0, len(names))
	for _, name := range names {
		item := completionItem{Label: name, Kind: kindProperty, Detail: "attribute"}
		if gomponents.IsBooleanAttribute(name) {
			item.Detail = "boolean attribute"
		} else {
			// Land the caret between the quotes.
			item.InsertText = name + `="$1"`
		}
		out = append(out, item)
	}
	return out
}

// componentNames returns the exported functions in the buffer that return Node,
// which is what an uppercase tag calls.
//
// This scans lines rather than parsing. Completion runs while the user is
// halfway through typing `<Car`, so the buffer almost never parses at the
// moment the answer is needed — parsing it would mean offering no components
// exactly when they are being asked for.
func componentNames(src string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range componentDecl.FindAllStringSubmatch(src, -1) {
		if name := m[1]; !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// componentDecl matches `func Name(...) Node {` on a single line, including a
// qualified return type such as `g.Node`.
var componentDecl = regexp.MustCompile(`(?m)^func\s+([A-Z][A-Za-z0-9_]*)\s*\([^)]*\)\s*(?:[A-Za-z_][A-Za-z0-9_]*\.)?Node\b`)
