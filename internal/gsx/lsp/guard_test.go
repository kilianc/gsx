package lsp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// The case that cost 33 hours of CPU: work that never returns must not take the
// proxy's message pump with it.
func TestRunGuardedSurvivesAHang(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	start := time.Now()
	_, err := runGuarded("compiling page.gsx", 50*time.Millisecond, func() (int, error) {
		<-release
		return 0, nil
	})
	if err == nil {
		t.Fatal("want an error when the work hangs")
	}
	if !strings.Contains(err.Error(), "did not finish") {
		t.Errorf("error = %q, want it to mention the budget", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s to give up; the budget is not being enforced", elapsed)
	}
}

// A panic would otherwise unwind through the pump goroutine and kill the whole
// LSP session.
func TestRunGuardedSurvivesAPanic(t *testing.T) {
	_, err := runGuarded("compiling page.gsx", time.Second, func() (int, error) {
		panic("boom")
	})
	if err == nil {
		t.Fatal("want an error when the work panics")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("error = %q, want it to mention the panic", err)
	}
}

// The guard must be invisible on the happy path, and must not reshape a real
// error into an internal one — a parse error still has to reach the caller so
// it can be shown as a diagnostic.
func TestRunGuardedPassesResultsThrough(t *testing.T) {
	got, err := runGuarded("x", time.Second, func() (string, error) {
		return "value", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}

	sentinel := errors.New("page.gsx:2:14: unexpected `<` in <p>")
	if _, err := runGuarded("x", time.Second, func() (string, error) {
		return "", sentinel
	}); !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want the caller's own error", err)
	}
}

// Both entry points must actually work through the guard.
func TestSafeCompileAndFormatRealWork(t *testing.T) {
	src := []byte("package p\n\nfunc F() Node { return <p>hi</p> }\n")

	goSrc, _, err := safeCompile("page.gsx", src)
	if err != nil {
		t.Fatalf("safeCompile: %v", err)
	}
	if !strings.Contains(string(goSrc), "package p") {
		t.Errorf("goSrc = %q", goSrc)
	}

	out, err := safeFormat("page.gsx", src)
	if err != nil {
		t.Fatalf("safeFormat: %v", err)
	}
	if !strings.Contains(string(out), "<p>hi</p>") {
		t.Errorf("formatted = %q", out)
	}
}

// Formatting runs the same parser on the same goroutine as compiling, so the
// buffer that wedged a session has to be survivable through this door too.
func TestSafeFormatRejectsAStrayLessThan(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := safeFormat("page.gsx", []byte("package p\n\nfunc F() Node { return <p>a < b</p> }\n"))
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want an error for a stray `<`")
		}
		if !strings.Contains(err.Error(), "unexpected `<`") {
			t.Errorf("error = %q, want the parse error", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("formatting hung on a stray `<`")
	}
}

// The whole failure, driven through the proxy the way an editor drives it.
//
// The unit tests above cover the parser and the guard separately; this is the
// seam that actually broke. A buffer containing a stray `<` — which is what a
// half-typed tag looks like between two keystrokes — has to come back, be
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
