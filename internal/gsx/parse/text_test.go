package parse

import "testing"

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// A run with no line break is content the author typed inline, so it is
		// preserved byte for byte — that is what keeps `<b>a</b> then <i>b</i>`
		// from losing its spaces.
		{"single line", "Hello", "Hello"},
		{"single line keeps spaces", " Hello ", " Hello "},
		{"single line keeps inner spaces", "a  b", "a  b"},

		// Runs spanning lines are layout: trim, drop blanks, join with one space.
		{"indented single line", "\n\tHello\n", "Hello"},
		{"wrapped sentence", "\n\ta\n\tb\n", "a b"},
		{"blank lines dropped", "\n\ta\n\n\n\tb\n", "a b"},
		{"leading text on first line", "a\n\tb\n", "a b"},
		{"trailing text on last line", "\n\ta\n\tb", "a b"},
		{"crlf", "\r\n\ta\r\n\tb\r\n", "a b"},
		{"lone cr", "\r\ta\r\tb\r", "a b"},
		{"tabs become spaces mid-line", "\na\tb\n", "a b"},

		// Whitespace-only runs across a line break are pure indentation.
		{"whitespace only", "\n\t\t\n", ""},
		{"newline only", "\n", ""},

		// A whitespace-only run with no line break is a real space.
		{"spaces without newline", "   ", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeText(tt.in); got != tt.want {
				t.Errorf("normalizeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecodeEntities(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Tom &amp; Jerry", "Tom & Jerry"},
		{"&lt;3", "<3"},
		{"&gt;", ">"},
		{"&quot;q&quot;", `"q"`},
		{"&#65;&#66;", "AB"},
		{"&#x41;&#X42;", "AB"},
		{"caf&eacute;", "café"},
		{"&nbsp;", " "},
		{"no entities here", "no entities here"},

		// Without a terminating semicolon there is no reference. This is where
		// GSX deliberately differs from legacy HTML parsing: `&notice` must not
		// become `¬ice`, and a query string must survive intact.
		{"&notice", "&notice"},
		{"?a=1&next=2", "?a=1&next=2"},
		{"AT&T", "AT&T"},
		{"&", "&"},
		{"&;", "&;"},
		{"&#;", "&#;"},
		{"&#x;", "&#x;"},

		// An unknown but well-formed reference is left alone rather than mangled.
		{"&notarealentity;", "&notarealentity;"},

		// A decoded ampersand must not cascade into a second decode pass.
		{"&amp;amp;", "&amp;"},
	}
	for _, tt := range tests {
		if got := decodeEntities(tt.in); got != tt.want {
			t.Errorf("decodeEntities(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsCommentOnly(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"/* hi */", true},
		{"  /* hi */  ", true},
		{"// hi\n", true},
		{"/* a */ /* b */", true},
		{"\n\t/* multi\n line */\n", true},
		{"", true},
		{"   ", true},
		{"x", false},
		{"/* hi */ x", false},
		{"x /* hi */", false},
		{`"/* not a comment */"`, false},
	}
	for _, tt := range tests {
		if got := isCommentOnly(tt.in); got != tt.want {
			t.Errorf("isCommentOnly(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
