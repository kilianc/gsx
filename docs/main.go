// Command docs renders the GSX documentation site to static HTML.
//
// The site is written in GSX, so building it is also the largest end-to-end
// test of the compiler: a change that breaks real-world usage breaks the docs
// build in CI.
//
//	go run ./docs -out ./docs/dist
//
// The playground page needs a wasm build of the compiler alongside the HTML,
// so the build also produces gsx.wasm and copies in the loader that ships with
// the Go toolchain. Pass -wasm=false to skip that while editing prose; the
// playground is then the only page that will not work.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kilianc/gsx/docs/site"
)

// staticDir holds the files served verbatim: the playground's JavaScript, and
// anything else that is not rendered from GSX.
const staticDir = "docs/static"

func main() {
	out := flag.String("out", "docs/dist", "directory to write the site into")
	serve := flag.String("serve", "", "if set, serve the site on this address instead of writing it")
	wasm := flag.Bool("wasm", true, "build the playground's wasm bundle")
	flag.Parse()

	if *serve != "" {
		// Pages are rendered per request, but the playground's assets are
		// built once so `gsx dev` picks up prose edits without rebuilding a
		// 12MB binary on every keystroke.
		assets, err := os.MkdirTemp("", "gsx-docs-assets")
		if err != nil {
			fatal(err)
		}
		defer os.RemoveAll(assets)

		if err := assetsInto(assets, *wasm); err != nil {
			fatal(err)
		}
		if err := site.Serve(*serve, assets); err != nil {
			fatal(err)
		}
		return
	}

	if err := build(*out, *wasm); err != nil {
		fatal(err)
	}
}

func build(dir string, wasm bool) error {
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

	if err := assetsInto(dir, wasm); err != nil {
		return err
	}

	// GitHub Pages runs the output through Jekyll unless told not to, which
	// would silently drop any file or directory beginning with an underscore.
	return os.WriteFile(filepath.Join(dir, ".nojekyll"), nil, 0o644)
}

// assetsInto copies the static files into dir and, unless disabled, builds the
// playground bundle beside them.
func assetsInto(dir string, wasm bool) error {
	if err := copyDir(staticDir, dir); err != nil {
		return fmt.Errorf("copying %s: %w", staticDir, err)
	}
	if !wasm {
		return nil
	}
	if err := buildWasm(dir); err != nil {
		return fmt.Errorf("building playground: %w", err)
	}
	return nil
}

// buildWasm compiles cmd/gsx-wasm and copies in wasm_exec.js.
//
// The loader is version-locked to the toolchain that produced the binary, so
// it is taken from GOROOT rather than checked in — a copy that drifts from the
// compiler fails at runtime in ways that are tedious to diagnose.
func buildWasm(dir string) error {
	out := filepath.Join(dir, "gsx.wasm")

	// -s -w drop the symbol table and DWARF: about a third of the download,
	// and nothing in a browser reads them.
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", out, "./cmd/gsx-wasm")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	loader, err := wasmExecPath()
	if err != nil {
		return err
	}
	if err := copyFile(loader, filepath.Join(dir, "wasm_exec.js")); err != nil {
		return err
	}

	if fi, err := os.Stat(out); err == nil {
		fmt.Fprintf(os.Stderr, "docs: %s (%.1f MB)\n", out, float64(fi.Size())/(1<<20))
	}
	return nil
}

// wasmExecPath locates the loader, which moved in Go 1.24.
func wasmExecPath() (string, error) {
	root := runtime.GOROOT()
	if root == "" {
		b, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			return "", fmt.Errorf("locating GOROOT: %w", err)
		}
		root = strings.TrimSpace(string(b))
	}

	candidates := []string{
		filepath.Join(root, "lib", "wasm", "wasm_exec.js"), // Go >= 1.24
		filepath.Join(root, "misc", "wasm", "wasm_exec.js"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("wasm_exec.js not found under %s", root)
}

// copyDir copies src into dst recursively; the editor bundle lives in a
// vendor/ subdirectory and has to arrive with its structure intact.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from, to := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(to, 0o755); err != nil {
				return err
			}
			if err := copyDir(from, to); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "docs:", err)
	os.Exit(1)
}
