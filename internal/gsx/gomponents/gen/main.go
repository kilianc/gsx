// Command gen regenerates the HTML element and attribute tables in
// internal/gsx/gomponents from the gomponents source itself.
//
// Hand-maintaining those tables meant GSX knew about 22 of gomponents' 100+
// elements and silently degraded the rest to El("tag", ...). Reading the real
// package keeps the mapping exact and makes a dependency bump a one-command
// refresh.
//
// Usage:
//
//	go generate ./internal/gsx/gomponents
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const gomponentsModule = "maragu.dev/gomponents"

type tables struct {
	// elements maps an HTML tag name to the gomponents/html function for it.
	elements map[string]string
	// stringAttrs and boolAttrs map an HTML attribute name to its function.
	stringAttrs map[string]string
	boolAttrs   map[string]string
	// exports is every exported name in gomponents/html, used to decide whether
	// a bare identifier written by the user needs an `html.` qualifier.
	exports []string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, err := moduleDir(gomponentsModule)
	if err != nil {
		return err
	}
	htmlDir := filepath.Join(dir, "html")

	t := &tables{
		elements:    map[string]string{},
		stringAttrs: map[string]string{},
		boolAttrs:   map[string]string{},
	}
	if err := t.collect(htmlDir); err != nil {
		return err
	}
	if len(t.elements) == 0 || len(t.stringAttrs) == 0 {
		return fmt.Errorf("extracted nothing from %s; did the gomponents API change?", htmlDir)
	}

	src, err := t.render(dir)
	if err != nil {
		return err
	}
	return os.WriteFile("html_tables.go", src, 0o644)
}

func moduleDir(path string) (string, error) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", path).Output()
	if err != nil {
		return "", fmt.Errorf("locating %s: %w", path, err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("%s has no local directory; run `go mod download`", path)
	}
	return dir, nil
}

// collect walks every function in gomponents/html and classifies it by what it
// returns: g.El("tag", ...) is an element, g.Attr("name", v) a string attribute,
// g.Attr("name") a boolean one.
func (t *tables) collect(htmlDir string) error {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, htmlDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return err
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || !fd.Name.IsExported() {
					continue
				}
				t.exports = append(t.exports, fd.Name.Name)

				name, kind, ok := classify(fd)
				if !ok {
					continue
				}
				// Two functions producing the same HTML name in the same
				// category would make the emitted table depend on map
				// iteration order. gomponents avoids this with its El/Attr
				// suffixes (Style is the attribute, StyleEl the element), but
				// fail loudly rather than emit something non-deterministic.
				var into map[string]string
				switch kind {
				case kindElement:
					into = t.elements
				case kindStringAttr:
					into = t.stringAttrs
				case kindBoolAttr:
					into = t.boolAttrs
				}
				if prev, dup := into[name]; dup {
					return fmt.Errorf("%q is produced by both %s and %s; the table would be non-deterministic",
						name, prev, fd.Name.Name)
				}
				into[name] = fd.Name.Name
			}
		}
	}
	sort.Strings(t.exports)
	return nil
}

type kind int

const (
	kindElement kind = iota
	kindStringAttr
	kindBoolAttr
)

// classify inspects a function body of the shape
//
//	func Div(children ...g.Node) g.Node { return g.El("div", children...) }
//
// and reports the HTML name it produces. Functions that do not have exactly
// that shape — Doctype, Raw, and anything else hand-written — are skipped, so
// they keep whatever bespoke handling they already have.
func classify(fd *ast.FuncDecl) (name string, k kind, ok bool) {
	if fd.Body == nil || len(fd.Body.List) != 1 {
		return "", 0, false
	}
	ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", 0, false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", 0, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", 0, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", 0, false
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil || name == "" {
		return "", 0, false
	}

	switch sel.Sel.Name {
	case "El":
		return name, kindElement, true
	case "Attr":
		if len(call.Args) == 1 {
			return name, kindBoolAttr, true
		}
		return name, kindStringAttr, true
	}
	return "", 0, false
}

func (t *tables) render(modDir string) ([]byte, error) {
	version := filepath.Base(modDir) // e.g. "gomponents@v1.2.0"

	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by internal/gsx/gomponents/gen. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "// Source: %s\n\n", version)
	fmt.Fprintf(&b, "package gomponents\n\n")

	fmt.Fprintf(&b, "// htmlElements maps an HTML tag name to its gomponents/html constructor.\n")
	fmt.Fprintf(&b, "// A tag with no entry here is emitted as El(\"tag\", ...), which renders the\n")
	fmt.Fprintf(&b, "// same but forgoes the typed helper.\n")
	writeStringMap(&b, "htmlElements", t.elements)

	fmt.Fprintf(&b, "\n// htmlStringAttrs maps an HTML attribute name to its constructor.\n")
	writeStringMap(&b, "htmlStringAttrs", t.stringAttrs)

	fmt.Fprintf(&b, "\n// htmlBoolAttrs maps a valueless HTML attribute name to its constructor.\n")
	writeStringMap(&b, "htmlBoolAttrs", t.boolAttrs)

	fmt.Fprintf(&b, "\n// htmlExports is every exported name in gomponents/html. A bare identifier\n")
	fmt.Fprintf(&b, "// written inside a `{...}` splice is qualified with the html package prefix\n")
	fmt.Fprintf(&b, "// when it appears here.\n")
	fmt.Fprintf(&b, "var htmlExports = map[string]bool{\n")
	for _, name := range t.exports {
		fmt.Fprintf(&b, "\t%q: true,\n", name)
	}
	fmt.Fprintf(&b, "}\n")

	return format.Source(b.Bytes())
}

func writeStringMap(b *bytes.Buffer, name string, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintf(b, "var %s = map[string]string{\n", name)
	for _, k := range keys {
		fmt.Fprintf(b, "\t%q: %q,\n", k, m[k])
	}
	fmt.Fprintf(b, "}\n")
}
