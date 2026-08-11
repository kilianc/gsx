package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIsGSXURI(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{"file:///foo/bar.gsx", true},
		{"file:///foo/bar.gsx.go", false},
		{"file:///foo/bar.go", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isGSXURI(tt.uri); got != tt.want {
			t.Errorf("isGSXURI(%q) = %v, want %v", tt.uri, got, tt.want)
		}
	}
}

func TestGSXToGoURI(t *testing.T) {
	if got := gsxToGoURI("file:///foo.gsx"); got != "file:///foo.gsx.go" {
		t.Fatalf("got %q", got)
	}
}

func TestGoToGSXURI(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"file:///foo.gsx.go", "file:///foo.gsx"},
		{"file:///foo.go", "file:///foo.go"},
		{"file:///foo.gsx", "file:///foo.gsx"},
	}
	for _, tt := range tests {
		if got := goToGSXURI(tt.in); got != tt.want {
			t.Errorf("goToGSXURI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestURIRoundTrip(t *testing.T) {
	uri := "file:///workspace/page.gsx"
	goURI := gsxToGoURI(uri)
	back := goToGSXURI(goURI)
	if back != uri {
		t.Fatalf("round trip: %q -> %q -> %q", uri, goURI, back)
	}
}

func TestIdentAt(t *testing.T) {
	tests := []struct {
		line string
		ch   int
		want string
	}{
		{"hello world", 3, "hello"},
		{"hello world", 0, "hello"},
		{"hello world", 5, "hello"},
		{"hello world", 6, "world"},
		{"{myVar}", 4, "myVar"},
		{"  foo_bar123  ", 5, "foo_bar123"},
		{"", 0, ""},
		{"   ", 1, ""},
		{"a", 0, "a"},
		{"a", 1, "a"},
	}
	for _, tt := range tests {
		got := identAt(tt.line, tt.ch)
		if got != tt.want {
			t.Errorf("identAt(%q, %d) = %q, want %q", tt.line, tt.ch, got, tt.want)
		}
	}
}

func TestRewriteDidOpen(t *testing.T) {
	st := newState()

	msg := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/page.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       "package page\n\nfunc Page() Node {\n\treturn <div>hello</div>\n}\n",
			},
		})),
	})

	out, err := st.rewriteClientToGopls(msg)
	if err != nil {
		t.Fatalf("rewriteClientToGopls: %v", err)
	}

	var result rpcMsg
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	var params struct {
		TextDocument struct {
			URI        string `json:"uri"`
			LanguageID string `json:"languageId"`
			Text       string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(*result.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}

	if params.TextDocument.URI != "file:///test/page.gsx.go" {
		t.Errorf("URI = %q, want file:///test/page.gsx.go", params.TextDocument.URI)
	}
	if params.TextDocument.LanguageID != "go" {
		t.Errorf("languageId = %q, want go", params.TextDocument.LanguageID)
	}

	d := st.getDoc("file:///test/page.gsx")
	if d == nil {
		t.Fatal("doc state not stored")
	}
	if d.sourceMap == nil {
		t.Fatal("sourceMap is nil")
	}

	// Verify that a compile-success clears pending diagnostic.
	diag := st.popPendingDiagnostic()
	if diag == nil {
		t.Fatal("expected clear diagnostic notification")
	}
	var diagMsg rpcMsg
	if err := json.Unmarshal(diag, &diagMsg); err != nil {
		t.Fatalf("unmarshal diagnostic: %v", err)
	}
	var diagParams struct {
		URI         string `json:"uri"`
		Diagnostics []any  `json:"diagnostics"`
	}
	if err := json.Unmarshal(*diagMsg.Params, &diagParams); err != nil {
		t.Fatalf("unmarshal diagnostic params: %v", err)
	}
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected empty diagnostics on success, got %d", len(diagParams.Diagnostics))
	}
}

func TestRewriteDidOpen_CompileError(t *testing.T) {
	st := newState()

	msg := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/bad.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       "this is not valid go or gsx",
			},
		})),
	})

	_, err := st.rewriteClientToGopls(msg)
	if err != nil {
		t.Fatalf("rewriteClientToGopls: %v", err)
	}

	diag := st.popPendingDiagnostic()
	if diag == nil {
		t.Fatal("expected compile error diagnostic")
	}

	var diagMsg rpcMsg
	if err := json.Unmarshal(diag, &diagMsg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var p struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Message  string `json:"message"`
			Severity int    `json:"severity"`
			Source   string `json:"source"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(*diagMsg.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p.URI != "file:///test/bad.gsx" {
		t.Errorf("diagnostic URI = %q, want file:///test/bad.gsx", p.URI)
	}
	if len(p.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(p.Diagnostics))
	}
	if p.Diagnostics[0].Severity != 1 {
		t.Errorf("severity = %d, want 1 (error)", p.Diagnostics[0].Severity)
	}
	if p.Diagnostics[0].Source != "gsx" {
		t.Errorf("source = %q, want gsx", p.Diagnostics[0].Source)
	}
}

func TestRewriteDiagnostics(t *testing.T) {
	st := newState()

	// Set up a doc state with known content.
	gsxText := "package page\n\nfunc Page() Node {\n\treturn <div>hello</div>\n}\n"
	openMsg := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/page.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       gsxText,
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(openMsg); err != nil {
		t.Fatalf("didOpen: %v", err)
	}
	_ = st.popPendingDiagnostic()

	d := st.getDoc("file:///test/page.gsx")
	if d == nil {
		t.Fatal("doc not found")
	}

	// Simulate a diagnostic from gopls pointing at line 0, col 0 of the .gsx.go file.
	goplsDiag := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"uri": d.goURI,
			"diagnostics": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": 0, "character": 0},
					"end":   map[string]any{"line": 0, "character": 7},
				},
				"severity": 1,
				"message":  "test error",
			}},
		})),
	})

	out, err := st.rewriteGoplsToClient(goplsDiag)
	if err != nil {
		t.Fatalf("rewriteGoplsToClient: %v", err)
	}

	var result rpcMsg
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(*result.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if p.URI != "file:///test/page.gsx" {
		t.Errorf("diagnostic URI = %q, want .gsx URI", p.URI)
	}
}

func TestStripCapabilities(t *testing.T) {
	resp := map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync":       1,
			"executeCommandProvider": map[string]any{"commands": []string{"test"}},
			"inlayHintProvider":      true,
			"codeActionProvider":     true,
			"semanticTokensProvider": map[string]any{},
			"hoverProvider":          true,
		},
	}
	removed := stripCapabilities(resp)
	caps := resp["capabilities"].(map[string]any)

	for _, k := range []string{"executeCommandProvider", "inlayHintProvider", "codeActionProvider", "semanticTokensProvider"} {
		if _, ok := caps[k]; ok {
			t.Errorf("%s should have been removed", k)
		}
		if _, ok := removed[k]; !ok {
			t.Errorf("%s should be in removed map", k)
		}
	}
	if _, ok := caps["hoverProvider"]; !ok {
		t.Error("hoverProvider should not have been removed")
	}
}

func TestForceFullSync(t *testing.T) {
	resp := map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": 2,
		},
	}
	forceFullSync(resp)
	caps := resp["capabilities"].(map[string]any)
	sync, ok := caps["textDocumentSync"].(map[string]any)
	if !ok {
		t.Fatal("textDocumentSync not set to map")
	}
	if change, _ := sync["change"].(int); change != 1 {
		t.Errorf("change = %v, want 1", sync["change"])
	}
}

// --- helpers ---

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

func rawPtr(b []byte) *json.RawMessage {
	rm := json.RawMessage(b)
	return &rm
}

// A GSX parse error carries an exact offset, so its diagnostic must land on the
// offending character rather than at the top of the file.
func TestRewriteDidOpen_ParseErrorIsPositioned(t *testing.T) {
	st := newState()

	// The mismatched </span> starts at line 4 (0-based 3), byte 28 of that line.
	src := "package p\n\nfunc Bad() Node {\n\treturn <div class=\"card\">hi</span>\n}\n"

	msg := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri":        "file:///test/bad.gsx",
				"languageId": "gsx",
				"version":    1,
				"text":       src,
			},
		})),
	})

	if _, err := st.rewriteClientToGopls(msg); err != nil {
		t.Fatalf("rewriteClientToGopls: %v", err)
	}

	diag := st.popPendingDiagnostic()
	if diag == nil {
		t.Fatal("expected compile error diagnostic")
	}

	var diagMsg rpcMsg
	if err := json.Unmarshal(diag, &diagMsg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var p struct {
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
				End struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"end"`
			} `json:"range"`
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(*diagMsg.Params, &p); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if len(p.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(p.Diagnostics))
	}
	d := p.Diagnostics[0]

	if d.Range.Start.Line != 3 || d.Range.Start.Character != 28 {
		t.Errorf("start = %d:%d, want 3:28", d.Range.Start.Line, d.Range.Start.Character)
	}
	if d.Range.End.Character != 29 {
		t.Errorf("end character = %d, want 29 (non-empty range)", d.Range.End.Character)
	}
	if !strings.Contains(d.Message, "mismatched closing tag </span>") {
		t.Errorf("message = %q", d.Message)
	}
	// The snippet and path belong to the CLI renderer, not to an editor squiggle.
	if strings.Contains(d.Message, "^") || strings.Contains(d.Message, "bad.gsx") {
		t.Errorf("message should not repeat the path or snippet: %q", d.Message)
	}
}
