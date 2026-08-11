// Package dev implements `gsx dev`: a live-reload development server.
//
// The reload story for a Go server is not the JavaScript one. Editing a `.gsx`
// file has to regenerate `.gsx.go`, rebuild the binary, restart the process and
// only then tell the browser to reload — a page refresh against a stale process
// shows stale HTML. So this package supervises the application rather than just
// watching files, and proxies to it so the reload client can be injected
// without the application knowing anything about GSX.
package dev

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Change describes what a batch of filesystem events touched.
type Change struct {
	// GSX is true when any `.gsx` source changed and code must be regenerated.
	GSX bool
	// Go is true when any `.go` file changed, meaning the app must be rebuilt.
	// Regenerating `.gsx` also produces `.gsx.go`, so a GSX change implies this.
	Go bool
	// Paths lists the files that triggered the batch, for logging.
	Paths []string
}

// Watcher reports debounced batches of source changes under a root.
type Watcher struct {
	root     string
	debounce time.Duration
	ignore   func(string) bool
}

// NewWatcher watches root recursively. Directories that never contain source —
// version control, dependencies, editor state — are skipped, both to cut noise
// and because a `.git` directory alone can exhaust the OS watch limit.
func NewWatcher(root string, debounce time.Duration) *Watcher {
	return &Watcher{root: root, debounce: debounce, ignore: defaultIgnore}
}

func defaultIgnore(name string) bool {
	switch name {
	case "vendor", "node_modules", "testdata", "dist", "build", "tmp":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// Watch delivers a Change to out for each debounced batch, until ctx is done.
func (w *Watcher) Watch(ctx context.Context, out chan<- Change) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	if err := w.addTree(fsw, w.root); err != nil {
		return err
	}

	var (
		pending Change
		seen    = map[string]bool{}
		timer   = time.NewTimer(time.Hour)
	)
	timer.Stop()
	defer timer.Stop()

	flush := func() {
		if !pending.GSX && !pending.Go {
			return
		}
		select {
		case out <- pending:
		case <-ctx.Done():
		}
		pending = Change{}
		seen = map[string]bool{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			// A new directory has to be added explicitly; fsnotify is not
			// recursive. Without this, files created in a directory made after
			// startup are invisible.
			if ev.Has(fsnotify.Create) && isDir(ev.Name) {
				if !w.ignore(filepath.Base(ev.Name)) {
					_ = w.addTree(fsw, ev.Name)
				}
			}

			kind, interesting := classify(ev.Name)
			if !interesting || !ev.Has(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) {
				continue
			}
			switch kind {
			case changeGSX:
				pending.GSX = true
				pending.Go = true
			case changeGo:
				pending.Go = true
			}
			if !seen[ev.Name] {
				seen[ev.Name] = true
				pending.Paths = append(pending.Paths, ev.Name)
			}

			// Editors write a file in several operations, and one save can
			// produce a burst of events. Restart the timer so a batch settles
			// before triggering a rebuild.
			timer.Reset(w.debounce)

		case <-timer.C:
			flush()

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

type changeKind int

const (
	changeNone changeKind = iota
	changeGSX
	changeGo
)

func classify(path string) (changeKind, bool) {
	base := filepath.Base(path)

	// Editors save through temporary files: `.#foo.gsx`, `foo.gsx~`,
	// `.!1234!foo.gsx`, `foo.gsx.swp`. Reacting to those rebuilds against a
	// half-written file and fills the log with names the user never typed.
	if strings.HasPrefix(base, ".") || strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") {
		return changeNone, false
	}

	switch {
	case strings.HasSuffix(base, ".gsx"):
		return changeGSX, true

	case strings.HasSuffix(base, ".gsx.go"):
		// Our own output. The `.gsx` edit that produced it already queued a
		// rebuild; reacting to it as well would make every save rebuild twice
		// and reload the browser twice.
		return changeNone, false

	case strings.HasSuffix(base, ".go"):
		return changeGo, true
	}
	return changeNone, false
}

func (w *Watcher) addTree(fsw *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A directory that vanished mid-walk is not worth failing over.
			return nil //nolint:nilerr
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && w.ignore(d.Name()) {
			return filepath.SkipDir
		}
		return fsw.Add(path)
	})
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
