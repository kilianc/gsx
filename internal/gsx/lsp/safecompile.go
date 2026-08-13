package lsp

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kilianc/gsx/internal/gsx/compile"
)

// compileBudget bounds a single compile.
//
// A `.gsx` file compiles in well under a millisecond, so this is not a limit on
// large files — it is the point past which the compiler is certainly not
// working but looping. It is generous enough that no real file can reach it.
const compileBudget = 5 * time.Second

// compileFunc is the compiler's shape, taken as a parameter rather than read
// from a package variable.
//
// A swappable global would be the shorter way to make the watchdog testable,
// but a timed-out compile is abandoned and keeps running, so it would still be
// reading that global after the call returned — the race detector flags it, and
// it is the same class of problem this file exists to contain.
type compileFunc func(path string, src []byte) ([]byte, *compile.SourceMap, error)

// safeCompile runs the compiler under a time budget, and turns a panic into an
// error rather than letting it reach the goroutine that called us.
//
// The proxy compiles on the same goroutine that pumps every client message to
// gopls, so the compiler failing to return is not a lost file — it is a session
// that stops forwarding anything, with no error, no log line and no crash. The
// editor keeps talking to a proxy that will never answer again, and the process
// spins at 100% CPU until someone notices and kills it by hand. That is exactly
// how a stray `<` in child position cost 33 hours of CPU before it was fixed.
//
// Bounding the call means the worst a future compiler bug can do is drop
// language features for the one file that triggers it, with a diagnostic saying
// so. It cannot take the session down with it.
//
// One caveat, stated plainly because it is easy to assume otherwise: Go cannot
// kill a goroutine, so a compile that truly hangs is abandoned, not stopped, and
// keeps burning a core until the process exits. This bounds the damage; the
// parser's own progress invariant is what prevents it.
func safeCompile(path string, src []byte) ([]byte, *compile.SourceMap, error) {
	return safeCompileWithin(compile.CompileFileForLSP, path, src, compileBudget)
}

func safeCompileWithin(compileFn compileFunc, path string, src []byte, budget time.Duration) ([]byte, *compile.SourceMap, error) {
	type result struct {
		goSrc []byte
		sm    *compile.SourceMap
		err   error
	}

	// Buffered, so that if we time out and stop listening the compile goroutine
	// can still send and exit instead of blocking here forever.
	done := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: fmt.Errorf("internal error: the gsx compiler panicked (%v); this is a bug in gsx, please report it", r)}
			}
		}()
		goSrc, sm, err := compileFn(path, src)
		done <- result{goSrc, sm, err}
	}()

	timer := time.NewTimer(budget)
	defer timer.Stop()

	select {
	case r := <-done:
		return r.goSrc, r.sm, r.err
	case <-timer.C:
		return nil, nil, fmt.Errorf("internal error: compiling %s did not finish within %s; this is a bug in gsx, please report it", filepath.Base(path), budget)
	}
}
