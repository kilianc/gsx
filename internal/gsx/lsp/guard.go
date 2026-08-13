package lsp

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kilianc/gsx/internal/gsx/compile"
	gsxformat "github.com/kilianc/gsx/internal/gsx/format"
)

// guardBudget bounds a single parser-backed operation.
//
// A `.gsx` file compiles in well under a millisecond, so this is not a limit on
// large files — it is the point past which the work is certainly not
// progressing but looping. It is generous enough that no real file reaches it.
const guardBudget = 5 * time.Second

// runGuarded runs fn under a time budget, and turns a panic into an error
// rather than letting it reach the goroutine that called us.
//
// The proxy does this work on the same goroutine that pumps every client
// message to gopls, so an operation that fails to return is not a lost file —
// it is a session that stops forwarding anything, with no error, no log line
// and no crash. The editor keeps talking to a proxy that will never answer
// again, and the process spins at 100% CPU until someone notices and kills it
// by hand. That is exactly how a stray `<` in child position cost 33 hours of
// CPU before it was fixed.
//
// Bounding the call means the worst a future parser bug can do is drop one
// feature for the one file that triggers it, with a diagnostic saying so. It
// cannot take the session down with it.
//
// One caveat, stated plainly because it is easy to assume otherwise: Go cannot
// kill a goroutine, so work that truly hangs is abandoned, not stopped, and
// keeps burning a core until the process exits. This bounds the damage; the
// parser's own progress invariant is what prevents it.
//
// fn is a parameter rather than a swappable package variable so the tests can
// supply one that hangs or panics. A global would still be read by the
// abandoned goroutine after the call returned — the race detector flags it, and
// it is the same class of problem this file exists to contain.
func runGuarded[T any](what string, budget time.Duration, fn func() (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}

	// Buffered, so that if we time out and stop listening the abandoned
	// goroutine can still send and exit instead of blocking here forever.
	done := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				var zero T
				done <- result{zero, fmt.Errorf("internal error: %s panicked (%v); this is a bug in gsx, please report it", what, r)}
			}
		}()
		v, err := fn()
		done <- result{v, err}
	}()

	timer := time.NewTimer(budget)
	defer timer.Stop()

	select {
	case r := <-done:
		return r.val, r.err
	case <-timer.C:
		var zero T
		return zero, fmt.Errorf("internal error: %s did not finish within %s; this is a bug in gsx, please report it", what, budget)
	}
}

// compiled is runGuarded's single return value for a compile, which produces
// two.
type compiled struct {
	goSrc []byte
	sm    *compile.SourceMap
}

// safeCompile compiles a buffer for gopls under the guard.
func safeCompile(path string, src []byte) ([]byte, *compile.SourceMap, error) {
	c, err := runGuarded("compiling "+filepath.Base(path), guardBudget, func() (compiled, error) {
		goSrc, sm, err := compile.CompileFileForLSP(path, src)
		return compiled{goSrc, sm}, err
	})
	return c.goSrc, c.sm, err
}

// safeFormat formats a buffer under the guard.
//
// Formatting runs the same parser as compiling, on the same pump goroutine, so
// leaving it unguarded would leave the identical wedge reachable through
// textDocument/formatting instead of textDocument/didChange.
func safeFormat(path string, src []byte) ([]byte, error) {
	return runGuarded("formatting "+filepath.Base(path), guardBudget, func() ([]byte, error) {
		return gsxformat.Source(path, src)
	})
}
