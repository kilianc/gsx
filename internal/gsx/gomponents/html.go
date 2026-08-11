package gomponents

import "strings"

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
