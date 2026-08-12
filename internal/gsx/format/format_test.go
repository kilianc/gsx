package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func format(t *testing.T, src string) string {
	t.Helper()
	out, err := Source("f.gsx", []byte(src))
	if err != nil {
		t.Fatalf("Source: %v", err)
	}
	return string(out)
}

func TestFormatsGoCode(t *testing.T) {
	in := "package p\nimport  \"strings\"\nfunc   F( a string ,b bool )   Node {\nx:=strings.TrimSpace(a)\nreturn <p>{x}</p>\n}\n"
	want := `package p

import "strings"

func F(a string, b bool) Node {
	x := strings.TrimSpace(a)
	return <p>{x}</p>
}
`
	if got := format(t, in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// The markup's own shape is the author's; only its indentation moves.
func TestReindentsMarkupWithoutReshapingIt(t *testing.T) {
	in := "package p\n\nfunc F() Node {\nreturn (\n\t\t\t\t<section class=\"card\">\n\t\t\t\t\t<h2>hi</h2>\n\t\t\t\t</section>\n)\n}\n"
	want := `package p

func F() Node {
	return (
		<section class="card">
			<h2>hi</h2>
		</section>
	)
}
`
	if got := format(t, in); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// The parentheses around a multi-line tag are load-bearing: without them Go
// inserts a semicolon after `return` and the code no longer parses. They must
// survive formatting.
func TestPreservesParenWrappedForm(t *testing.T) {
	got := format(t, "package p\n\nfunc F() Node {\n\treturn (\n\t\t<div>\n\t\t\t<p>hi</p>\n\t\t</div>\n\t)\n}\n")
	if !strings.Contains(got, "return (\n") {
		t.Errorf("parentheses were dropped:\n%s", got)
	}
	if !strings.Contains(got, "\n\t)\n") {
		t.Errorf("closing parenthesis was dropped:\n%s", got)
	}
}

func TestLeavesSingleLineTagsAlone(t *testing.T) {
	got := format(t, "package p\n\nfunc F() Node {\n\treturn <div class=\"x\">hi</div>\n}\n")
	if !strings.Contains(got, `return <div class="x">hi</div>`) {
		t.Errorf("got:\n%s", got)
	}
}

func TestFormattingIsIdempotent(t *testing.T) {
	inputs := []string{
		"package p\nfunc F() Node { return <p>hi</p> }\n",
		"package p\n\nfunc F() Node {\nreturn (\n<div>\n<p>a</p>\n</div>\n)\n}\n",
		"package p\n\nfunc F(ok bool) Node {\n\treturn <div>{If(ok, <p>y</p>)}</div>\n}\n",
		"package p\n\nvar x = P{\n\tA: \"a\",\n\tBody: (\n\t\t<div>b</div>\n\t),\n}\n",
	}
	for _, in := range inputs {
		once := format(t, in)
		twice := format(t, once)
		if once != twice {
			t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", once, twice)
		}
	}
}

// Formatting is cosmetic: it must never change what the file means.
func TestPreservesTagContentExactly(t *testing.T) {
	in := "package p\n\nfunc F() Node {\n\treturn <pre>{`line one\n\tline two\nline three`}</pre>\n}\n"
	got := format(t, in)
	if !strings.Contains(got, "line one\n\tline two\nline three") {
		t.Errorf("raw string content was altered:\n%q", got)
	}
}

func TestReportsParseErrors(t *testing.T) {
	_, err := Source("page.gsx", []byte("package p\n\nfunc F() Node {\n\treturn <div>hi</span>\n}\n"))
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "mismatched closing tag") {
		t.Errorf("got %v", err)
	}
}

func TestReportsInvalidGo(t *testing.T) {
	if _, err := Source("page.gsx", []byte("package p\n\nfunc F( {\n")); err == nil {
		t.Fatal("want an error for unparseable Go")
	}
}

// Every .gsx file in the repository must already be formatted, so that running
// the formatter is never a surprise in review.
func TestRepositoryIsFormatted(t *testing.T) {
	root := "../../.."
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "testdata", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".gsx") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		out, err := Source(path, src)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		if string(out) != string(src) {
			t.Errorf("%s is not formatted; run `gsx fmt -w ./...`", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
