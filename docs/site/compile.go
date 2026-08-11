package site

import (
	"fmt"
	"strings"

	"github.com/kilianc/gsx/pkg/gsx"
	g "maragu.dev/gomponents"
)

// compileSnippet compiles a documentation snippet with the real compiler and
// returns the generated Go, minus the boilerplate a reader does not need.
//
// Compiling for real means a snippet cannot drift from what GSX actually
// produces: if the compiler changes, the docs change with it, and a snippet
// that stops compiling fails the docs build.
func compileSnippet(src string) (string, error) {
	src = strings.TrimSpace(src)

	// Snippets are written as bare declarations. Wrap them in a package so the
	// compiler sees a whole file, and import the packages the examples use.
	full := "package doc\n\nimport (\n\t\"strings\"\n\t\"time\"\n)\n\nvar _ = strings.TrimSpace\nvar _ = time.Now\n\n" + src + "\n"

	out, err := gsx.CompileFile("doc.gsx", []byte(full))
	if err != nil {
		return "", err
	}
	return trimGenerated(string(out)), nil
}

// trimGenerated strips the generated header, package clause, import block and
// the placeholders compileSnippet added, leaving only the declarations.
func trimGenerated(s string) string {
	lines := strings.Split(s, "\n")

	// Drop everything through the end of the import block.
	start := 0
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == ")" {
			start = i + 1
			break
		}
		if strings.HasPrefix(t, "package ") {
			start = i + 1
		}
	}

	var out []string
	for _, l := range lines[start:] {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "var _ = ") {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// Compiled renders a snippet beside the Go that GSX actually generates for it.
func Compiled(src string) g.Node {
	out, err := compileSnippet(src)
	if err != nil {
		// Fail the docs build rather than shipping a page that quietly omits
		// the output it promises.
		panic(fmt.Sprintf("docs: snippet failed to compile: %v\n\n%s", err, src))
	}
	return Split(GSX(src), Out("generated go", out))
}

// CompiledBelow is Compiled for snippets too wide to read side by side.
func CompiledBelow(src string) g.Node {
	out, err := compileSnippet(src)
	if err != nil {
		panic(fmt.Sprintf("docs: snippet failed to compile: %v\n\n%s", err, src))
	}
	return g.Group{GSX(src), Out("generated go", out)}
}
