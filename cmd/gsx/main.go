package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/compile"
	"github.com/kilianc/gsx/internal/gsx/outfile"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: gsx [flags] [paths...]")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Generates one *.gsx.go file next to each *.gsx source.")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Paths behave like Go patterns:")
		_, _ = fmt.Fprintln(os.Stderr, "  - ./...        recurse from cwd")
		_, _ = fmt.Fprintln(os.Stderr, "  - ./dir        only that directory (non-recursive)")
		_, _ = fmt.Fprintln(os.Stderr, "  - ./dir/...    recurse from that directory")
		_, _ = fmt.Fprintln(os.Stderr, "  - ./file.gsx   only that file")
		flag.PrintDefaults()
	}
	rootFlag := flag.String("root", "", "module root (defaults to auto-detected go.mod parent from cwd)")
	dirFlag := flag.String("dir", "", "if set, only generate for this directory (non-recursive). Useful with go:generate.")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	root := *rootFlag
	if root == "" {
		root, err = findModuleRoot(cwd)
		if err != nil {
			fatal(err)
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fatal(err)
	}

	if strings.TrimSpace(*dirFlag) != "" && flag.NArg() != 0 {
		fatal(fmt.Errorf("gsx: cannot use -dir with positional paths"))
	}

	if strings.TrimSpace(*dirFlag) != "" {
		dir := *dirFlag
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		dir, err = filepath.Abs(dir)
		if err != nil {
			fatal(err)
		}
		if err := generateDir(root, dir); err != nil {
			fatal(err)
		}
		return
	}

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	paths, err := collectGSXPaths(cwd, patterns)
	if err != nil {
		fatal(err)
	}
	if len(paths) == 0 {
		return
	}

	sort.Strings(paths)
	var allErr error
	for _, pth := range paths {
		if err := generateFile(root, pth); err != nil {
			allErr = errors.Join(allErr, err)
		}
	}
	if allErr != nil {
		fatal(allErr)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func findModuleRoot(start string) (string, error) {
	d := start
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("could not find go.mod above %s", start)
		}
		d = parent
	}
}

func discoverGSX(root string) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			name := de.Name()
			if name == "vendor" || name == "node_modules" || name == "cursor-extension" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(de.Name(), ".gsx") {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			out[filepath.Dir(abs)] = append(out[filepath.Dir(abs)], abs)
		}
		return nil
	})
	return out, err
}

func generateDir(moduleRoot, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".gsx") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)

	for _, pth := range paths {
		if err := generateFile(moduleRoot, pth); err != nil {
			return err
		}
	}
	return nil
}

func generateFile(moduleRoot, pth string) error {
	b, err := os.ReadFile(pth)
	if err != nil {
		return err
	}
	src, err := compile.CompileFile(pth, b)
	if err != nil {
		return fmt.Errorf("%s: %w", pth, err)
	}
	outPath := pth + ".go"
	if err := outfile.WriteGeneratedFile(outPath, src); err != nil {
		return err
	}
	return nil
}

func collectGSXPaths(cwd string, patterns []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string

	add := func(p string) error {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(cwd, abs)
		}
		abs, err := filepath.Abs(abs)
		if err != nil {
			return err
		}
		if !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
		return nil
	}

	for _, raw := range patterns {
		pat := strings.TrimSpace(raw)
		if pat == "" {
			continue
		}

		// Recursive pattern: <dir>/...
		if strings.HasSuffix(pat, "/...") || pat == "./..." || pat == "..." {
			base := strings.TrimSuffix(pat, "...")
			base = strings.TrimSuffix(base, "/")
			if base == "" {
				base = "."
			}
			dir := base
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(cwd, dir)
			}
			dir, err := filepath.Abs(dir)
			if err != nil {
				return nil, err
			}
			if err := walkGSX(dir, func(p string) error { return add(p) }); err != nil {
				return nil, err
			}
			continue
		}

		// Non-recursive: file.gsx or directory.
		target := pat
		if !filepath.IsAbs(target) {
			target = filepath.Join(cwd, target)
		}
		target, err := filepath.Abs(target)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(target)
		if err != nil {
			return nil, err
		}
		if st.IsDir() {
			entries, err := os.ReadDir(target)
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.HasSuffix(e.Name(), ".gsx") {
					if err := add(filepath.Join(target, e.Name())); err != nil {
						return nil, err
					}
				}
			}
			continue
		}
		if !strings.HasSuffix(target, ".gsx") {
			return nil, fmt.Errorf("gsx: not a .gsx file: %s", target)
		}
		if err := add(target); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func walkGSX(root string, add func(string) error) error {
	return filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			name := de.Name()
			if name == "vendor" || name == "node_modules" || name == "cursor-extension" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(de.Name(), ".gsx") {
			return add(path)
		}
		return nil
	})
}
