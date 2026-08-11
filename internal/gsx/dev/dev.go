package dev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options configures `gsx dev`.
type Options struct {
	// Root is the directory watched for changes.
	Root string
	// Run is the command that builds and starts the application.
	Run string
	// AppAddr is where the application listens.
	AppAddr string
	// Addr is where the dev server listens; this is the address to open.
	Addr string
	// Debounce is how long to wait for filesystem events to settle.
	Debounce time.Duration
	// Generate regenerates every `.gsx` under Root. It is injected so the dev
	// loop does not depend on the compiler package directly, which keeps this
	// package testable without compiling real sources.
	Generate func() error
	// Log receives progress output.
	Log io.Writer
}

// Run starts the dev server and blocks until ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	if opts.Log == nil {
		opts.Log = io.Discard
	}
	if opts.Debounce <= 0 {
		opts.Debounce = 120 * time.Millisecond
	}

	target, err := url.Parse("http://" + opts.AppAddr)
	if err != nil {
		return fmt.Errorf("invalid app address %q: %w", opts.AppAddr, err)
	}

	broker := NewBroker()
	runner := &Runner{Command: opts.Run, Dir: opts.Root, Addr: opts.AppAddr, Out: opts.Log}
	defer runner.Stop()

	srv := &http.Server{Addr: opts.Addr, Handler: NewProxy(target, broker)}
	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", opts.Addr, err)
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(opts.Log, "gsx dev: server: %v\n", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logf(opts.Log, "listening on http://%s → %s", opts.Addr, opts.AppAddr)

	// Build once up front so the first page load is already current.
	rebuild(ctx, opts, runner, broker, Change{GSX: true, Go: true}, true)

	events := make(chan Change, 1)
	watcher := NewWatcher(opts.Root, opts.Debounce)
	go func() {
		if err := watcher.Watch(ctx, events); err != nil && ctx.Err() == nil {
			fmt.Fprintf(opts.Log, "gsx dev: watch: %v\n", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ch := <-events:
			rebuild(ctx, opts, runner, broker, ch, false)
		}
	}
}

// rebuild runs one change through the pipeline: regenerate, restart, reload.
//
// A failure at any stage is published to the browser as an overlay and does not
// stop the loop — the next save should get a chance to fix it.
func rebuild(ctx context.Context, opts Options, runner *Runner, broker *Broker, ch Change, initial bool) {
	if ctx.Err() != nil {
		return
	}
	start := time.Now()

	if !initial {
		logf(opts.Log, "changed: %s", describe(opts.Root, ch.Paths))
	}

	if ch.GSX && opts.Generate != nil {
		if err := opts.Generate(); err != nil {
			logf(opts.Log, "generate failed:\n%v", err)
			broker.Publish(Event{Kind: "error", Message: err.Error()})
			return
		}
	}

	if err := runner.Restart(ctx); err != nil {
		if ctx.Err() != nil {
			return
		}
		logf(opts.Log, "restart failed: %v", err)
		broker.Publish(Event{Kind: "error", Message: err.Error()})
		return
	}

	logf(opts.Log, "ready in %s", time.Since(start).Round(time.Millisecond))
	broker.Publish(Event{Kind: "reload"})
}

// describe renders the changed paths relative to the root, keeping the log
// readable when a single save touches several files.
func describe(root string, paths []string) string {
	const max = 3
	rel := make([]string, 0, len(paths))
	for _, p := range paths {
		if r, err := filepath.Rel(root, p); err == nil {
			p = r
		}
		rel = append(rel, p)
	}
	if len(rel) > max {
		return fmt.Sprintf("%s (+%d more)", strings.Join(rel[:max], ", "), len(rel)-max)
	}
	return strings.Join(rel, ", ")
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "gsx dev: "+format+"\n", args...)
}

// DefaultRoot resolves the directory to watch.
func DefaultRoot(dir string) (string, error) {
	if dir == "" {
		return os.Getwd()
	}
	return filepath.Abs(dir)
}
