package parse

import (
	"strings"
	"testing"
)

// FuzzRewriteTagsMakesProgress hunts for input the parser cannot get past.
//
// RewriteTags returning an error is a perfectly good outcome — most random
// bytes are not valid `.gsx` — so this does not assert success. It asserts the
// one thing that is never acceptable: tripping the progress invariant, which
// means the parser would have spun on that byte forever had the invariant not
// caught it. A failure here is a hang found before it reaches an editor.
//
// The seed corpus runs on every `go test`, so the known-bad shapes stay pinned
// even when nobody is fuzzing. To go looking for new ones:
//
//	go test ./internal/gsx/parse/ -run FuzzRewriteTagsMakesProgress -fuzz . -fuzztime 2m
func FuzzRewriteTagsMakesProgress(f *testing.F) {
	// Every seed is a shape that has stalled the parser or plausibly could:
	// `<` in text, unterminated constructs, and the delimiters the loops key on.
	for _, seed := range []string{
		"package p\nvar x = <p>a < b</p>\n",
		"package p\nvar x = <p>5 <= 6</p>\n",
		"package p\nvar x = <p>a <-ch</p>\n",
		"package p\nvar x = <>a < b</>\n",
		"package p\nvar x = <p><",
		"package p\nvar x = <p attr=<",
		"package p\nvar x = <p {...a}<",
		"package p\nvar x = <p>{a < b}</p>\n",
		"package p\nvar x = <p>{/* < */}</p>\n",
		"package p\nvar x = <p>{\"<\"}</p>\n",
		"package p\nvar x = <p>&lt;</p>\n",
		"package p\nvar x = <p></",
		"package p\nvar x = <p attr=\"",
		"package p\nvar x = <p>{",
		"package p\nvar x = a < b\n",
		"package p\nvar x = `<`\n",
		"package p\n// <\n",
		"package p\n/* < */\n",
		"<",
		"<<<",
		"{",
		"</>",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		_, _, err := RewriteTags("f.gsx", []byte(src))
		if err != nil && strings.Contains(err.Error(), "consumed no input") {
			t.Fatalf("parser stalled on %q: %v", src, err)
		}
	})
}
