package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: playground [flags]")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Watches ./playground/page.gsx and re-runs the gsx generator on changes.")
	}
	interval := flag.Duration("interval", 300*time.Millisecond, "watch polling interval")
	flag.Parse()

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	if err := watchAndGenerate(*interval); err != nil {
		fatal(err)
	}
}

func watchAndGenerate(interval time.Duration) error {
	root, err := findModuleRoot(".")
	if err != nil {
		return err
	}
	target := filepath.Join(root, "playground", "page.gsx")

	var lastHash [32]byte
	var have bool

	for {
		src, err := os.ReadFile(target)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "playground: read error: %v\n", err)
			time.Sleep(interval)
			continue
		}
		h := sha256Sum(src)
		if !have || h != lastHash {
			lastHash = h
			have = true

			cmd := exec.Command("go", "run", "./cmd/gsx", "./playground")
			cmd.Dir = root
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "playground: gsx generate failed: %v\n", err)
			}
		}

		time.Sleep(interval)
	}
}

func findModuleRoot(start string) (string, error) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
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

func sha256Sum(b []byte) [32]byte {
	// local tiny helper to avoid pulling in fsnotify; polling is enough for v0.
	return sha256.Sum256(b)
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
