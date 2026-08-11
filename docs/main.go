// Command docs renders the GSX documentation site to static HTML.
//
// The site is written in GSX, so building it is also the largest end-to-end
// test of the compiler: a change that breaks real-world usage breaks the docs
// build in CI.
//
//	go run ./docs -out ./docs/dist
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kilianc/gsx/docs/site"
)

func main() {
	out := flag.String("out", "docs/dist", "directory to write the site into")
	serve := flag.String("serve", "", "if set, serve the site on this address instead of writing it")
	flag.Parse()

	if *serve != "" {
		if err := site.Serve(*serve); err != nil {
			fatal(err)
		}
		return
	}

	if err := build(*out); err != nil {
		fatal(err)
	}
}

func build(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	pages := site.Pages()
	for _, p := range pages {
		html, err := site.Render(p, pages)
		if err != nil {
			return fmt.Errorf("rendering %s: %w", p.Slug, err)
		}
		path := filepath.Join(dir, p.Slug+".html")
		if err := os.WriteFile(path, html, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "docs: %s\n", path)
	}

	// GitHub Pages runs the output through Jekyll unless told not to, which
	// would silently drop any file or directory beginning with an underscore.
	if err := os.WriteFile(filepath.Join(dir, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "docs:", err)
	os.Exit(1)
}
