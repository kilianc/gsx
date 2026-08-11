package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kilianc/gsx/internal/gsx/compile"
	"github.com/kilianc/gsx/internal/gsx/lsp"
	"github.com/kilianc/gsx/internal/gsx/outfile"
	"github.com/kilianc/gsx/internal/gsx/parse"
)

func main() {
	// Subcommand: `gsx lsp`
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		if err := lspMain(os.Args[2:]); err != nil {
			fatal(err)
		}
		return
	}

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
	dirFlag := flag.String("dir", "", "if set, only generate for this directory (non-recursive). Useful with go:generate.")
	checkFlag := flag.Bool("check", false, "do not write; exit non-zero if any generated file is out of date")
	flag.Parse()

	cwd, err := os.Getwd()
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
		paths, err := dirGSXPaths(dir)
		if err != nil {
			fatal(err)
		}
		if err := run(paths, *checkFlag); err != nil {
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
	if err := run(paths, *checkFlag); err != nil {
		fatal(err)
	}
}

// run compiles every path. In check mode it writes nothing and instead reports
// the files whose generated output differs from what is on disk, so CI can
// verify checked-in `*.gsx.go` files are current.
func run(paths []string, check bool) error {
	sort.Strings(paths)

	var allErr error
	var stale []string
	for _, pth := range paths {
		src, err := compileFile(pth)
		if err != nil {
			allErr = errors.Join(allErr, err)
			continue
		}
		outPath := pth + ".go"

		if check {
			existing, err := os.ReadFile(outPath)
			if err != nil || !bytes.Equal(existing, src) {
				stale = append(stale, outPath)
			}
			continue
		}

		fmt.Fprintf(os.Stderr, "gsx: %s\n", pth)
		if err := outfile.WriteGeneratedFile(outPath, src); err != nil {
			allErr = errors.Join(allErr, err)
		}
	}

	if len(stale) > 0 {
		for _, p := range stale {
			fmt.Fprintf(os.Stderr, "gsx: out of date: %s\n", p)
		}
		allErr = errors.Join(allErr, fmt.Errorf("%d generated file(s) out of date; run `gsx ./...`", len(stale)))
	}
	return allErr
}

func lspMain(args []string) error {
	fs := flag.NewFlagSet("lsp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	goplsLog := fs.String("goplsLog", "", "path to gopls log file (passed as -logfile)")
	goplsRPCTrace := fs.Bool("goplsRPCTrace", false, "enable gopls RPC tracing (passed as -rpc.trace)")
	goplsRemote := fs.String("goplsRemote", "", "gopls remote (passed as -remote)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var goplsArgs []string
	if *goplsLog != "" {
		goplsArgs = append(goplsArgs, "-logfile", *goplsLog)
	}
	if *goplsRPCTrace {
		goplsArgs = append(goplsArgs, "-rpc.trace")
	}
	if *goplsRemote != "" {
		goplsArgs = append(goplsArgs, "-remote", *goplsRemote)
	}
	return lsp.Run(context.Background(), os.Stdin, os.Stdout, goplsArgs)
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func dirGSXPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
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
	return paths, nil
}

func compileFile(pth string) ([]byte, error) {
	b, err := os.ReadFile(pth)
	if err != nil {
		return nil, err
	}
	src, err := compile.CompileFile(pth, b)
	if err != nil {
		// A parse error already renders as `path:line:col: msg` with a source
		// snippet, so prefixing it again would just repeat the path.
		var pe *parse.Error
		if errors.As(err, &pe) {
			return nil, err
		}
		return nil, fmt.Errorf("%s: %w", pth, err)
	}
	return src, nil
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
