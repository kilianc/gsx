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

// gopls cannot format a .gsx buffer, so the proxy answers formatting itself and
// must not forward the request.
func TestFormattingIsAnsweredByTheProxy(t *testing.T) {
	st := newState()

	unformatted := "package p\n\nfunc F() Node {\nreturn <p>hi</p>\n}\n"
	open := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///test/page.gsx", "languageId": "gsx", "version": 1, "text": unformatted,
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(open); err != nil {
		t.Fatal(err)
	}
	_ = st.popPendingDiagnostic()

	req := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "textDocument/formatting",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{"uri": "file:///test/page.gsx"},
			"options":      map[string]any{"tabSize": 4, "insertSpaces": false},
		})),
	})

	out, err := st.rewriteClientToGopls(req)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Errorf("request was forwarded to gopls: %s", out)
	}

	resp := st.popPendingDiagnostic()
	if resp == nil {
		t.Fatal("no response produced")
	}

	var msg struct {
		ID     int `json:"id"`
		Result []struct {
			NewText string `json:"newText"`
			Range   struct {
				Start struct{ Line, Character int } `json:"start"`
				End   struct{ Line, Character int } `json:"end"`
			} `json:"range"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ID != 42 {
		t.Errorf("id = %d, want 42", msg.ID)
	}
	if len(msg.Result) != 1 {
		t.Fatalf("got %d edits, want 1", len(msg.Result))
	}
	if !strings.Contains(msg.Result[0].NewText, "\treturn <p>hi</p>") {
		t.Errorf("edit did not format the body:\n%s", msg.Result[0].NewText)
	}
	// The edit must cover the whole buffer.
	if msg.Result[0].Range.Start.Line != 0 || msg.Result[0].Range.End.Line < 5 {
		t.Errorf("range %+v does not cover the document", msg.Result[0].Range)
	}
}

// Formatting source that does not parse must produce no edits rather than
// mangling it; the diagnostics already explain the problem.
func TestFormattingBrokenSourceMakesNoEdits(t *testing.T) {
	st := newState()

	open := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///test/bad.gsx", "languageId": "gsx", "version": 1,
				"text": "package p\n\nfunc F() Node {\n\treturn <div>hi</span>\n}\n",
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(open); err != nil {
		t.Fatal(err)
	}
	_ = st.popPendingDiagnostic()

	req := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0", ID: 7, Method: "textDocument/formatting",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{"uri": "file:///test/bad.gsx"},
		})),
	})
	if _, err := st.rewriteClientToGopls(req); err != nil {
		t.Fatal(err)
	}

	resp := st.popPendingDiagnostic()
	if resp == nil {
		t.Fatal("no response produced")
	}
	var msg struct {
		Result any `json:"result"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Result != nil {
		t.Errorf("result = %v, want null for unparseable source", msg.Result)
	}
}

// publishDiagnostics replaces the whole set for a URI. gopls publishes for the
// same file moments after GSX does, so without merging, a parse error appears
// and is immediately wiped — which is what happened in a real editor: the
// language server reported the error and the editor showed none.
func TestGSXDiagnosticsSurviveGoplsPublish(t *testing.T) {
	st := newState()

	open := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///test/page.gsx", "languageId": "gsx", "version": 1,
				"text": "package demo\n\nfunc F() Node {\n\treturn <div>hi</span>\n}\n",
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(open); err != nil {
		t.Fatal(err)
	}

	// The GSX parse error is published first.
	first := st.popPendingDiagnostic()
	if first == nil {
		t.Fatal("no GSX diagnostic published")
	}
	if !strings.Contains(string(first), "mismatched closing tag") {
		t.Fatalf("first publish is not the parse error: %s", first)
	}

	// Now gopls publishes its own set for the generated file.
	goplsMsg := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"uri": "file:///test/page.gsx.go",
			"diagnostics": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": 0, "character": 0},
					"end":   map[string]any{"line": 0, "character": 1},
				},
				"severity": 1,
				"message":  "something from gopls",
			}},
		})),
	})
	out, err := st.rewriteGoplsToClient(goplsMsg)
	if err != nil {
		t.Fatal(err)
	}

	var p struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Message string `json:"message"`
		} `json:"diagnostics"`
	}
	var msg rpcMsg
	if err := json.Unmarshal(out, &msg); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(*msg.Params, &p); err != nil {
		t.Fatal(err)
	}

	if p.URI != "file:///test/page.gsx" {
		t.Errorf("uri = %q, want the .gsx URI", p.URI)
	}
	var sawGSX, sawGopls bool
	for _, d := range p.Diagnostics {
		if strings.Contains(d.Message, "mismatched closing tag") {
			sawGSX = true
		}
		if strings.Contains(d.Message, "something from gopls") {
			sawGopls = true
		}
	}
	if !sawGSX {
		t.Error("the GSX parse error was dropped when gopls published")
	}
	if !sawGopls {
		t.Error("gopls diagnostics were dropped")
	}
}

// A position that cannot be mapped must be answered, not forwarded. Forwarding
// sends gopls a .gsx URI it was never told about, and it replies with an error
// that the editor shows as "Request textDocument/hover failed" on every hover.
func TestUnmappablePositionIsAnsweredNotForwarded(t *testing.T) {
	st := newState()

	// A file that does not compile has no source map, so nothing maps.
	open := mustMarshal(t, rpcMsg{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: rawPtr(mustMarshal(t, map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///test/bad.gsx", "languageId": "gsx", "version": 1,
				"text": "package demo\n\nfunc F() Node {\n\treturn <div>hi</span>\n}\n",
			},
		})),
	})
	if _, err := st.rewriteClientToGopls(open); err != nil {
		t.Fatal(err)
	}
	_ = st.popPendingDiagnostic()

	for _, method := range []string{"textDocument/hover", "textDocument/definition", "textDocument/completion"} {
		t.Run(method, func(t *testing.T) {
			req := mustMarshal(t, rpcMsg{
				JSONRPC: "2.0", ID: 99, Method: method,
				Params: rawPtr(mustMarshal(t, map[string]any{
					"textDocument": map[string]any{"uri": "file:///test/bad.gsx"},
					"position":     map[string]any{"line": 3, "character": 6},
				})),
			})
			out, err := st.rewriteClientToGopls(req)
			if err != nil {
				t.Fatal(err)
			}
			if out != nil {
				t.Errorf("request was forwarded to gopls: %s", out)
			}
			resp := st.popPendingDiagnostic()
			if resp == nil {
				t.Fatal("no response produced; the client would hang or error")
			}
			if !strings.Contains(string(resp), `"id":99`) {
				t.Errorf("response does not answer the request: %s", resp)
			}
		})
	}
}

// The virtual Go view for a file that fails to compile must still declare the
// package the rest of the directory uses. A placeholder of `package p` made
// gopls report "found packages main and p" for every file in the folder, and
// those diagnostics buried the parse error that caused them.
func TestPackageNameOf(t *testing.T) {
	tests := []struct{ src, want string }{
		{"package main\n\nfunc F() {}", "main"},
		{"// a comment\n\npackage ui\n", "ui"},
		{"\n\npackage  spaced \n", "spaced"},
		{"no package clause at all", "p"},
		{"", "p"},
	}
	for _, tt := range tests {
		if got := packageNameOf(tt.src); got != tt.want {
			t.Errorf("packageNameOf(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}
