package lsp

import (
	"encoding/json"
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

// The whole failure, driven through the proxy the way an editor drives it.
//
// The unit tests above cover the parser and the watchdog separately; this is
// the seam that actually broke. A buffer containing a stray `<` — which is what
// a half-typed tag looks like between two keystrokes — has to come back, be
// reported as a diagnostic, and leave the pump able to handle the next message.
// Before the fix this call never returned, and every message behind it in the
// queue was never forwarded to gopls again.
func TestProxySurvivesAStrayLessThanInTheBuffer(t *testing.T) {
	st := newState()

	open := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/page.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       "package page\n\nfunc Page() Node {\n\treturn <p>a < b</p>\n}\n",
			},
		})),
	})

	done := make(chan error, 1)
	go func() {
		_, err := st.rewriteClientToGopls(open)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("rewriteClientToGopls: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the proxy hung on a stray `<`; the pump would never forward another message")
	}

	// The failure has to reach the editor as a diagnostic. Dropping it silently
	// is how the original bug stayed invisible for 33 hours.
	//
	// Decoded rather than substring-matched: json.Marshal HTML-escapes `<` into
	// a `<` sequence, so a match against the raw bytes would miss the very
	// character under test.
	var published struct {
		Params struct {
			URI         string `json:"uri"`
			Diagnostics []struct {
				Message string `json:"message"`
				Range   struct {
					Start struct {
						Line      int `json:"line"`
						Character int `json:"character"`
					} `json:"start"`
				} `json:"range"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	found := false
	for _, msg := range st.takePendingForClient() {
		var m rpcMsg
		if err := json.Unmarshal(msg, &m); err != nil || m.Method != "textDocument/publishDiagnostics" {
			continue
		}
		if err := json.Unmarshal(msg, &published); err != nil {
			t.Fatalf("decode diagnostic: %v", err)
		}
		found = true
	}
	if !found {
		t.Fatal("no diagnostic published for the failing buffer")
	}
	if len(published.Params.Diagnostics) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(published.Params.Diagnostics))
	}

	d := published.Params.Diagnostics[0]
	if !strings.Contains(d.Message, "unexpected `<`") {
		t.Errorf("message = %q, want it to name the stray `<`", d.Message)
	}
	// It must point at the `<` itself — line 4 of the source, 0-based 3.
	if d.Range.Start.Line != 3 || d.Range.Start.Character != 13 {
		t.Errorf("range starts at %d:%d, want 3:13 (the `<`)", d.Range.Start.Line, d.Range.Start.Character)
	}

	// And the session must still be alive: the next message still gets handled.
	next := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/ok.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       "package page\n\nfunc OK() Node {\n\treturn <p>hi</p>\n}\n",
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(next); err != nil {
		t.Fatalf("proxy did not survive: %v", err)
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
