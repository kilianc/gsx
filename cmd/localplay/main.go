// Command localplay regenerates localplay/page.gsx as you edit it.
//
// It is the playground for people working on GSX, as opposed to the one on the
// documentation site, which is for people evaluating it. The difference is
// which compiler runs: the site ships a prebuilt wasm bundle of the committed
// compiler, so it cannot show you the effect of a change you have not pushed.
// This runs the working tree.
//
// localplay/page.gsx is also a real file on disk, which the browser cannot
// offer — that is what makes it usable for checking that gopls resolves
// go-to-definition and hover through a .gsx file.
//
//	go run ./cmd/localplay
//
// For a full application with live reload, use `gsx dev` instead; this only
// regenerates one file and supervises nothing.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kilianc/gsx/internal/gsx/module"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "Usage: localplay [flags]")
		_, _ = fmt.Fprintln(os.Stderr, "")
		_, _ = fmt.Fprintln(os.Stderr, "Watches ./localplay/page.gsx and re-runs the gsx generator on changes.")
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
	root, err := module.FindModuleRoot(".")
	if err != nil {
		return err
	}
	target := filepath.Join(root, "localplay", "page.gsx")

	var lastHash [32]byte
	var have bool

	for {
		src, err := os.ReadFile(target)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "localplay: read error: %v\n", err)
			time.Sleep(interval)
			continue
		}
		h := sha256Sum(src)
		if !have || h != lastHash {
			lastHash = h
			have = true

			cmd := exec.Command("go", "run", "./cmd/gsx", "./localplay")
			cmd.Dir = root
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "localplay: gsx generate failed: %v\n", err)
			}
		}

		time.Sleep(interval)
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
