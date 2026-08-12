package gomponents

import (
	"sort"
	"strings"
)

//go:generate go run ./gen

// jsxAttrAliases maps JSX attribute spellings to their HTML names.
//
// JSX renames the attributes that collide with JavaScript reserved words and
// camel-cases the rest. GSX has no such collisions — `class` and `for` are
// perfectly good attribute names here — but accepting both spellings is the
// point of being JSX-compatible: pasted JSX should compile, and muscle memory
// should not produce a silently wrong attribute.
var jsxAttrAliases = map[string]string{
	"classname":           "class",
	"htmlfor":             "for",
	"tabindex":            "tabindex",
	"readonly":            "readonly",
	"maxlength":           "maxlength",
	"minlength":           "minlength",
	"colspan":             "colspan",
	"rowspan":             "rowspan",
	"autocomplete":        "autocomplete",
	"autofocus":           "autofocus",
	"autoplay":            "autoplay",
	"crossorigin":         "crossorigin",
	"datetime":            "datetime",
	"enctype":             "enctype",
	"formaction":          "formaction",
	"formenctype":         "formenctype",
	"formmethod":          "formmethod",
	"formnovalidate":      "formnovalidate",
	"formtarget":          "formtarget",
	"novalidate":          "novalidate",
	"playsinline":         "playsinline",
	"popovertarget":       "popovertarget",
	"popovertargetaction": "popovertargetaction",
	"referrerpolicy":      "referrerpolicy",
	"srcset":              "srcset",
}

// jsxAttrSpellings are the camel-cased names to offer in completion. The alias
// map above is keyed lower-case for case-insensitive lookup, which is the wrong
// spelling to suggest to someone typing.
var jsxAttrSpellings = []string{
	"className", "htmlFor", "tabIndex", "readOnly", "maxLength", "minLength",
	"colSpan", "rowSpan", "autoComplete", "autoFocus", "autoPlay", "crossOrigin",
	"dateTime", "encType", "formAction", "formEncType", "formMethod",
	"formNoValidate", "formTarget", "noValidate", "playsInline", "popoverTarget",
	"popoverTargetAction", "referrerPolicy", "srcSet",
}

// canonicalAttr resolves an attribute name as written to its HTML name.
//
// Lookup is case-insensitive because HTML attribute names are, which also makes
// `className`, `classname` and `CLASSNAME` all land on `class`.
func canonicalAttr(key string) string {
	lower := strings.ToLower(key)
	if name, ok := jsxAttrAliases[lower]; ok {
		return name
	}
	// `data-*` and `aria-*` are open-ended and pass through as written, but
	// still get lowercased so `dataFoo` and `data-foo` agree.
	return lower
}

// htmlElementFunc returns the gomponents/html constructor for an HTML tag, or
// "" when the tag has no typed helper and should be emitted via El.
func htmlElementFunc(tag string) string {
	return htmlElements[strings.ToLower(tag)]
}

// htmlStringAttrFunc returns the constructor for a value-bearing attribute.
func htmlStringAttrFunc(key string) string {
	return htmlStringAttrs[canonicalAttr(key)]
}

// htmlBoolAttrFunc returns the constructor for a valueless attribute.
func htmlBoolAttrFunc(key string) string {
	return htmlBoolAttrs[canonicalAttr(key)]
}

// ElementNames returns every HTML tag GSX maps to a typed constructor, sorted.
//
// Exported for editor completion: the language server offers these after `<`.
func ElementNames() []string { return sortedKeys(htmlElements) }

// AttributeNames returns every attribute name GSX knows, sorted, including the
// JSX spellings so completion can offer `className` alongside `class`.
func AttributeNames() []string {
	seen := map[string]bool{}
	for k := range htmlStringAttrs {
		seen[k] = true
	}
	for k := range htmlBoolAttrs {
		seen[k] = true
	}
	for _, jsx := range jsxAttrSpellings {
		seen[jsx] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// IsBooleanAttribute reports whether an attribute takes no value, so completion
// can insert `disabled` rather than `disabled=""`.
func IsBooleanAttribute(name string) bool { return htmlBoolAttrs[canonicalAttr(name)] != "" }

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
