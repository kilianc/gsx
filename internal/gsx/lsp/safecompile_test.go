package lsp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kilianc/gsx/internal/gsx/compile"
)

// The case that cost 33 hours of CPU: a compiler that never returns must not
// take the proxy's message pump with it.
func TestSafeCompileSurvivesAHang(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	hang := func(string, []byte) ([]byte, *compile.SourceMap, error) {
		<-release
		return nil, nil, nil
	}

	start := time.Now()
	_, _, err := safeCompileWithin(hang, "page.gsx", nil, 50*time.Millisecond)
	if err == nil {
		t.Fatal("want an error when the compiler hangs")
	}
	if !strings.Contains(err.Error(), "did not finish") {
		t.Errorf("error = %q, want it to mention the budget", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s to give up; the budget is not being enforced", elapsed)
	}
}

// A panic in the compiler would otherwise unwind through the pump goroutine and
// kill the whole LSP session.
func TestSafeCompileSurvivesAPanic(t *testing.T) {
	boom := func(string, []byte) ([]byte, *compile.SourceMap, error) {
		panic("boom")
	}

	_, _, err := safeCompileWithin(boom, "page.gsx", nil, time.Second)
	if err == nil {
		t.Fatal("want an error when the compiler panics")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("error = %q, want it to mention the panic", err)
	}
}

// The guardrail must be invisible on the happy path.
func TestSafeCompilePassesResultsThrough(t *testing.T) {
	want := []byte("package p\n")
	ok := func(string, []byte) ([]byte, *compile.SourceMap, error) {
		return want, nil, nil
	}
	got, _, err := safeCompileWithin(ok, "page.gsx", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("goSrc = %q, want %q", got, want)
	}

	// A normal parse error still has to reach the caller unchanged, so it can be
	// shown as a diagnostic rather than swallowed as an internal failure.
	sentinel := errors.New("page.gsx:2:14: unexpected `<` in <p>")
	failing := func(string, []byte) ([]byte, *compile.SourceMap, error) {
		return nil, nil, sentinel
	}
	if _, _, err := safeCompileWithin(failing, "page.gsx", nil, time.Second); !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the compiler's own error", err)
	}
}

// The real compiler must still work through the wrapper.
func TestSafeCompileRealCompiler(t *testing.T) {
	goSrc, _, err := safeCompile("page.gsx", []byte("package p\n\nfunc F() Node { return <p>hi</p> }\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goSrc), "package p") {
		t.Errorf("goSrc = %q", goSrc)
	}
}
