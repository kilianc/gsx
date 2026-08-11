package compile

import "testing"

// The pretty-printer collapses generated Go onto one line and re-indents it to
// the call site. Doing either inside a literal silently rewrites the program's
// data — an embedded SQL statement, JSON body, script block or code sample.
func TestCollapseOutsideLiterals(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a   b", "a b"},
		{"a\n\tb", "a b"},
		{"f(\n\ta,\n\tb,\n)", "f( a, b, )"},

		// Literal contents are untouched.
		{"f(`a\nb`)", "f(`a\nb`)"},
		{"f(`a  b`)", "f(`a  b`)"},
		{`f("a  b")`, `f("a  b")`},
		{`f("a\tb")`, `f("a\tb")`},
		{"f(' ')", "f(' ')"},

		// Whitespace around a literal still collapses.
		{"f(  `a\nb`  ,  1  )", "f( `a\nb` , 1 )"},

		// A backtick inside an interpreted string does not open a raw string,
		// so the whitespace after it still collapses.
		{"f(\"`\")  ;  g()", "f(\"`\") ; g()"},

		// An escaped quote does not end the literal.
		{`f("a \" b  c")`, `f("a \" b  c")`},
	}
	for _, tt := range tests {
		if got := collapseOutsideLiterals(tt.in); got != tt.want {
			t.Errorf("collapseOutsideLiterals(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}

func TestIndentCodeSkipsRawStrings(t *testing.T) {
	in := "f(\n`raw\nlines`,\n1,\n)"
	want := "\tf(\n\t`raw\nlines`,\n\t1,\n\t)"
	if got := indentCode(in, "\t", true); got != want {
		t.Errorf("indentCode\n got %q\nwant %q", got, want)
	}
}

func TestLiteralLen(t *testing.T) {
	tests := []struct {
		in   string
		i    int
		want int
	}{
		{`"abc"`, 0, 5},
		{`"a\"b"`, 0, 6},
		{"`a\nb`", 0, 5},
		{`'x'`, 0, 3},
		{`'\''`, 0, 4},
		{`"unterminated`, 0, 13},
		{"`unterminated", 0, 13},
	}
	for _, tt := range tests {
		if got := literalLen(tt.in, tt.i); got != tt.want {
			t.Errorf("literalLen(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
