package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kilianc/gsx/internal/gsx/compile"
)

var update = flag.Bool("update", false, "rewrite golden files instead of comparing against them")

// TestCompile compiles every `e2e/*.gsx` fixture and compares the result against
// the checked-in `*.gsx.go` file.
//
// Those generated files are part of this package, so a golden that does not
// build fails the package build before this test even runs. That makes the
// generated file both the golden and a compile check.
func TestCompile(t *testing.T) {
	for _, src := range fixtures(t, ".gsx") {
		t.Run(fixtureName(src), func(t *testing.T) {
			in, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := compile.CompileFile(src, in)
			if err != nil {
				t.Fatalf("compile %s: %v", src, err)
			}
			assertGolden(t, src+".go", string(got))
		})
	}
}

// TestRenderHTML renders every registered fixture and compares the HTML against
// its `<name>.html.out` golden.
func TestRenderHTML(t *testing.T) {
	names := make([]string, 0, len(GSXFunctions))
	for name := range GSXFunctions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			var sb strings.Builder
			if err := GSXFunctions[name]().Render(&sb); err != nil {
				t.Fatalf("render %s: %v", name, err)
			}
			assertGolden(t, name+".html.out", sb.String())
		})
	}
}

// TestNoOrphanGoldens catches `*.html.out` files whose fixture was renamed or
// stopped registering itself, which would otherwise silently stop being checked.
func TestNoOrphanGoldens(t *testing.T) {
	for _, golden := range fixtures(t, ".html.out") {
		name := strings.TrimSuffix(filepath.Base(golden), ".html.out")
		if _, ok := GSXFunctions[name]; !ok {
			t.Errorf("%s has no fixture registered in GSXFunctions[%q]", filepath.Base(golden), name)
		}
	}
}

// TestEveryFixtureCompiles guards against a `.gsx` file that produces no
// `.gsx.go`, which would mean it is never compiled or rendered by anything.
func TestEveryFixtureCompiles(t *testing.T) {
	for _, src := range fixtures(t, ".gsx") {
		if _, err := os.Stat(src + ".go"); err != nil {
			t.Errorf("%s has no generated %s — run `make gen`", filepath.Base(src), filepath.Base(src)+".go")
		}
	}
}

func fixtures(t *testing.T, suffix string) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		// `.gsx` must not also match `.gsx.go` / `.gsx.out`.
		if suffix == ".gsx" && filepath.Ext(strings.TrimSuffix(e.Name(), suffix)) != "" {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("no %s fixtures found", suffix)
	}
	return out
}

func fixtureName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".gsx")
}

func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run `go test ./e2e -update`): %v", path, err)
	}
	if string(want) == got {
		return
	}
	t.Errorf("%s is out of date (run `go test ./e2e -update`)\n%s", path, diff(string(want), got))
}

// diff renders a minimal line-oriented diff. It is only used to make test
// failures readable, so a naive first/last-mismatch window is enough.
func diff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")

	first := 0
	for first < len(wl) && first < len(gl) && wl[first] == gl[first] {
		first++
	}
	lastW, lastG := len(wl)-1, len(gl)-1
	for lastW > first && lastG > first && wl[lastW] == gl[lastG] {
		lastW--
		lastG--
	}

	var sb strings.Builder
	sb.WriteString("--- want\n+++ got\n")
	for i := first; i <= lastW && i < len(wl); i++ {
		sb.WriteString("-" + wl[i] + "\n")
	}
	for i := first; i <= lastG && i < len(gl); i++ {
		sb.WriteString("+" + gl[i] + "\n")
	}
	return sb.String()
}
