// Command gen regenerates the yaegi symbol tables in
// internal/gsx/playground/symbols.
//
// The playground interprets the compiler's output rather than building it, so
// every package that generated code may reference has to be handed to the
// interpreter as a table of reflect.Values. yaegi ships such a table for the
// whole standard library, but importing it links every package it covers:
// that build is 39MB where this one is 12MB, because the linker cannot prove
// any of it unreachable. Extracting only the packages the playground actually
// exposes keeps the wasm binary a third of the size.
//
// It is also the sandbox. Interpreted code can only reach what appears here,
// so leaving out os, net, and os/exec is what makes running a stranger's code
// in the reader's own browser safe — those identifiers do not exist in the
// interpreter's universe.
//
// Usage:
//
//	go generate ./internal/gsx/playground
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/traefik/yaegi/extract"
)

// packages is every import the playground makes available to interpreted code.
//
// gomponents and gomponents/html are what the compiler emits. The rest is the
// small slice of the standard library that markup realistically reaches for:
// formatting a price, upper-casing a label, sorting rows. Adding to this list
// widens the sandbox, so add deliberately.
var packages = []string{
	"maragu.dev/gomponents",
	"maragu.dev/gomponents/html",
	"fmt",
	"strings",
	"strconv",
	"sort",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := "symbols"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	e := extract.Extractor{Dest: "symbols"}
	for _, pkg := range packages {
		name := strings.ReplaceAll(strings.TrimPrefix(pkg, "maragu.dev/"), "/", "_")
		path := filepath.Join(dir, name+".go")

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := e.Extract(pkg, "symbols", f); err != nil {
			f.Close()
			return fmt.Errorf("extracting %s: %w", pkg, err)
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "gen: %s\n", path)
	}
	return nil
}
