package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kilianc/gsx/internal/gsx/compile"
	"github.com/kilianc/gsx/internal/gsx/parse"
)

// Run proxies LSP traffic between the editor (stdin/stdout) and gopls, rewriting .gsx files to
// generated virtual Go files and mapping diagnostics back to .gsx.
func Run(ctx context.Context, stdin io.Reader, stdout io.Writer, goplsArgs []string) error {
	p, err := startGopls(goplsArgs)
	if err != nil {
		return err
	}
	defer p.GracefulShutdown()

	clientR := bufio.NewReader(stdin)
	goplsR := bufio.NewReader(p.stdout)

	st := newState()

	// client -> gopls
	errCh := make(chan error, 2)
	go func() {
		for {
			msg, err := ReadMessage(clientR)
			if err != nil {
				if err == io.EOF {
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
			out, err := st.rewriteClientToGopls(msg)
			if err != nil {
				out = msg
			}
			// Anything the proxy answered itself goes straight back.
			for _, pending := range st.takePendingForClient() {
				_ = WriteMessage(stdout, pending)
			}
			// A nil result means the proxy handled the request and gopls must
			// not see it.
			if out == nil {
				continue
			}
			if err := WriteMessage(p.stdin, out); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// gopls -> client
	go func() {
		for {
			msg, err := ReadMessage(goplsR)
			if err != nil {
				if err == io.EOF {
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
			out, err := st.rewriteGoplsToClient(msg)
			if err != nil {
				out = msg
			}
			if err := WriteMessage(stdout, out); err != nil {
				errCh <- err
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if err == nil {
			return nil
		}
		return err
	}
}

type rpcMsg struct {
	JSONRPC string           `json:"jsonrpc,omitempty"`
	ID      any              `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  *json.RawMessage `json:"params,omitempty"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   any              `json:"error,omitempty"`
}

type docState struct {
	gsxURI string
	goURI  string

	gsxText string
	goText  string

	sourceMap *compile.SourceMap
}

type state struct {
	mu   sync.Mutex
	docs map[string]*docState // gsx uri -> state

	// initialize request id from client -> gopls; used to rewrite initialize response.
	initializeID any

	// Messages the proxy itself owes the client: diagnostics it produced, and
	// responses to requests gopls must not see.
	pendingClient [][]byte

	// gsxDiags holds the compile diagnostics GSX owns, per .gsx URI.
	//
	// publishDiagnostics replaces the whole set for a URI, so gopls publishing
	// for the same file would otherwise wipe a GSX parse error the moment it
	// arrived — which is exactly when the user needs to see it.
	gsxDiags map[string][]any
}

func newState() *state {
	return &state{
		docs:     map[string]*docState{},
		gsxDiags: map[string][]any{},
	}
}

// setGSXDiagnostics records the diagnostics GSX owns for a document and queues
// the merged set for the client.
func (s *state) setGSXDiagnostics(uri string, diags []any) {
	s.mu.Lock()
	if len(diags) == 0 {
		delete(s.gsxDiags, uri)
	} else {
		s.gsxDiags[uri] = diags
	}
	s.mu.Unlock()

	s.queueForClient(publishDiagnostics(uri, s.diagnosticsFor(uri, nil)))
}

// diagnosticsFor merges GSX's diagnostics with a set from gopls.
func (s *state) diagnosticsFor(uri string, fromGopls []any) []any {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]any, 0, len(s.gsxDiags[uri])+len(fromGopls))
	out = append(out, s.gsxDiags[uri]...)
	out = append(out, fromGopls...)
	return out
}

func (s *state) setPendingDiagnostic(msg []byte) { s.queueForClient(msg) }
func (s *state) setPendingResponse(msg []byte)   { s.queueForClient(msg) }

func (s *state) queueForClient(msg []byte) {
	if msg == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingClient = append(s.pendingClient, msg)
}

// popPendingDiagnostic returns the next message the proxy owes the client, or
// nil. It is a convenience for tests, which deal in one message at a time.
func (s *state) popPendingDiagnostic() []byte {
	msgs := s.takePendingForClient()
	if len(msgs) == 0 {
		return nil
	}
	// Put anything beyond the first back, preserving order.
	if len(msgs) > 1 {
		s.mu.Lock()
		s.pendingClient = append(msgs[1:], s.pendingClient...)
		s.mu.Unlock()
	}
	return msgs[0]
}

func (s *state) takePendingForClient() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.pendingClient
	s.pendingClient = nil
	return out
}

func (s *state) getDoc(gsxURI string) *docState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.docs[gsxURI]
}

func (s *state) upsertDoc(d *docState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[d.gsxURI] = d
}

func (s *state) deleteDoc(gsxURI string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, gsxURI)
}

func isGSXURI(uri string) bool { return strings.HasSuffix(uri, ".gsx") }

func gsxToGoURI(gsxURI string) string {
	// Match the on-disk generated filename to avoid duplicate decls in gopls:
	//   foo.gsx -> foo.gsx.go
	return gsxURI + ".go"
}

func goToGSXURI(goURI string) string {
	if strings.HasSuffix(goURI, ".gsx.go") {
		return strings.TrimSuffix(goURI, ".go")
	}
	return goURI
}

// mapPositionGoToSrc maps a position in generated Go (line/character 0-based) back to rewritten gsx.
func mapPositionGoToSrc(d *docState, line, ch int) (outLine, outCh int, ok bool) {
	if d == nil || d.sourceMap == nil {
		return 0, 0, false
	}
	p, ok := d.sourceMap.SourcePositionFromTarget(compile.Position{Line: line, Col: ch})
	if !ok {
		return 0, 0, false
	}
	return p.Line, p.Col, true
}

// mapPositionSrcToGo maps a position in rewritten gsx (line/character 0-based) into generated Go.
func mapPositionSrcToGo(d *docState, line, ch int) (outLine, outCh int, ok bool) {
	if d == nil || d.sourceMap == nil {
		return 0, 0, false
	}
	p, ok := d.sourceMap.TargetPositionFromSource(compile.Position{Line: line, Col: ch})
	if !ok {
		return 0, 0, false
	}
	return p.Line, p.Col, true
}

func lineAt(s string, line int) (string, bool) {
	if line < 0 {
		return "", false
	}
	lines := strings.Split(s, "\n")
	if line >= len(lines) {
		return "", false
	}
	return lines[line], true
}

func offsetToLineCol(s string, off int) (line, col int) {
	if off < 0 {
		off = 0
	}
	if off > len(s) {
		off = len(s)
	}
	line = 0
	lastNL := -1
	for i := 0; i < off; i++ {
		if s[i] == '\n' {
			line++
			lastNL = i
		}
	}
	col = off - (lastNL + 1)
	if col < 0 {
		col = 0
	}
	return
}

func identAt(lineText string, ch int) (ident string) {
	if ch < 0 {
		ch = 0
	}
	if ch > len(lineText) {
		ch = len(lineText)
	}
	isIdent := func(b byte) bool {
		return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
	}
	i := ch
	if i == len(lineText) && i > 0 {
		i--
	}
	if i < 0 || i >= len(lineText) {
		return ""
	}
	// If we're just after an ident, move left onto it.
	if !isIdent(lineText[i]) && i > 0 && isIdent(lineText[i-1]) {
		i--
	}
	if !isIdent(lineText[i]) {
		return ""
	}
	start := i
	for start > 0 && isIdent(lineText[start-1]) {
		start--
	}
	end := i + 1
	for end < len(lineText) && isIdent(lineText[end]) {
		end++
	}
	return lineText[start:end]
}

// mapDefinitionInsideBraces tries to map a definition request that targets an identifier inside a `{...}`
// block to the corresponding position in the generated Go view.
func mapDefinitionInsideBraces(d *docState, srcLine, srcCh int) (gl, gc int, ok bool) {
	if d == nil {
		return 0, 0, false
	}
	lt, okLine := lineAt(d.gsxText, srcLine)
	if !okLine {
		return 0, 0, false
	}
	if srcCh < 0 {
		srcCh = 0
	}
	if srcCh > len(lt) {
		srcCh = len(lt)
	}
	open := strings.LastIndex(lt[:srcCh], "{")
	if open < 0 {
		return 0, 0, false
	}
	closeRel := strings.Index(lt[srcCh:], "}")
	if closeRel < 0 {
		return 0, 0, false
	}
	closeIdx := srcCh + closeRel
	if closeIdx <= open {
		return 0, 0, false
	}
	expr := strings.TrimSpace(lt[open+1 : closeIdx])
	if expr == "" {
		return 0, 0, false
	}
	ident := identAt(lt, srcCh)
	if ident == "" {
		return 0, 0, false
	}

	// Use the sourcemap to get an approximate target line, then search within
	// a narrow window to avoid false matches from repeated identifiers.
	searchText := d.goText
	searchOffset := 0
	if tgtLine, _, smOK := mapPositionSrcToGo(d, srcLine, srcCh); smOK {
		goLines := strings.Split(d.goText, "\n")
		lo := tgtLine - 5
		if lo < 0 {
			lo = 0
		}
		hi := tgtLine + 6
		if hi > len(goLines) {
			hi = len(goLines)
		}
		for i := 0; i < lo; i++ {
			searchOffset += len(goLines[i]) + 1
		}
		searchText = strings.Join(goLines[lo:hi], "\n")
	}

	idx := strings.Index(searchText, expr)
	if idx < 0 {
		idx = strings.Index(searchText, ident)
		if idx < 0 {
			return 0, 0, false
		}
	} else {
		if ii := strings.Index(searchText[idx:], ident); ii >= 0 {
			idx += ii
		}
	}
	gl, gc = offsetToLineCol(d.goText, searchOffset+idx)
	return gl, gc, true
}

// gsxDiagnostic builds the diagnostic for a GSX compile failure.
//
// A *parse.Error knows the exact offset that failed, so the squiggle lands on
// the offending character instead of at the top of the file. Its rendered form
// repeats the path and a source snippet, which the editor already shows, so the
// message is reduced to the bare text.
func gsxDiagnostic(err error) []any {
	line, char := 0, 0
	msg := err.Error()

	var pe *parse.Error
	if errors.As(err, &pe) {
		l, c := pe.Position()
		// LSP positions are 0-based; parse.Error reports 1-based.
		line, char = l-1, c-1
		msg = pe.Msg
	}
	if line < 0 {
		line = 0
	}
	if char < 0 {
		char = 0
	}

	return []any{map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": line, "character": char},
			// A zero-width range renders as a caret; extend by one so the
			// editor draws a visible squiggle.
			"end": map[string]any{"line": line, "character": char + 1},
		},
		"severity": 1,
		"source":   "gsx",
		"message":  "GSX: " + msg,
	}}
}

// nullResponse answers a request with no result, which every LSP client treats
// as "nothing here" rather than as a failure.
func nullResponse(id any) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  nil,
	})
	return b
}

// publishDiagnostics builds the notification for a URI's full diagnostic set.
func publishDiagnostics(uri string, diags []any) []byte {
	if diags == nil {
		diags = []any{}
	}
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params":  map[string]any{"uri": uri, "diagnostics": diags},
	})
	return b
}

// packageNameOf reads the package clause from GSX source.
//
// When a file fails to compile, the virtual Go view handed to gopls still has
// to declare the same package as the rest of the directory. A placeholder of
// `package p` made gopls report "found packages main and p" for every file in
// the folder, and those diagnostics buried the parse error that caused it.
// mergedDiagnostics re-encodes a gopls diagnostic set for a .gsx URI with
// GSX's own diagnostics prepended.
func mergedDiagnostics[T any](s *state, uri string, fromGopls []T) []byte {
	converted := make([]any, 0, len(fromGopls))
	for _, d := range fromGopls {
		// Round-trip through JSON so gopls's fields survive untouched rather
		// than being narrowed to the subset this proxy models.
		b, err := json.Marshal(d)
		if err != nil {
			continue
		}
		var raw any
		if json.Unmarshal(b, &raw) == nil {
			converted = append(converted, raw)
		}
	}
	return publishDiagnostics(uri, s.diagnosticsFor(uri, converted))
}

func packageNameOf(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "package "); ok {
			if name := strings.Fields(rest); len(name) > 0 {
				return name[0]
			}
		}
	}
	return "p"
}

func makeDiagnosticNotification(uri string, line, char int, msg string) []byte {
	if line < 0 {
		line = 0
	}
	if char < 0 {
		char = 0
	}
	diag := map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]any{
			"uri": uri,
			"diagnostics": []map[string]any{{
				"range": map[string]any{
					"start": map[string]any{"line": line, "character": char},
					// A zero-width range renders as a caret; extend by one so the
					// editor draws a visible squiggle.
					"end": map[string]any{"line": line, "character": char + 1},
				},
				"severity": 1,
				"source":   "gsx",
				"message":  msg,
			}},
		},
	}
	b, _ := json.Marshal(diag)
	return b
}

// formattingResponse answers textDocument/formatting with a single edit
// replacing the whole document.
//
// A whole-document edit rather than a minimal diff because the formatter works
// on whole files, and the editor collapses it to the visible change anyway.
func formattingResponse(id any, d *docState) []byte {
	result := any(nil)

	if d != nil {
		if out, err := safeFormat(uriToPath(d.gsxURI), []byte(d.gsxText)); err == nil {
			if string(out) != d.gsxText {
				lines := strings.Count(d.gsxText, "\n") + 1
				result = []any{map[string]any{
					"range": map[string]any{
						"start": map[string]any{"line": 0, "character": 0},
						// One past the last line, character 0, covers the whole
						// buffer regardless of a trailing newline.
						"end": map[string]any{"line": lines, "character": 0},
					},
					"newText": string(out),
				}}
			} else {
				result = []any{}
			}
		}
		// On a parse error, leave result nil: the diagnostics already say why,
		// and reformatting broken source would be worse than doing nothing.
	}

	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	return b
}

// completionResponse answers a completion request with an incomplete list, so
// the editor re-asks as the user keeps typing.
func completionResponse(id any, items []completionItem) []byte {
	if items == nil {
		items = []completionItem{}
	}
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"isIncomplete": true,
			"items":        items,
		},
	})
	return b
}

// offsetOf converts a zero-based line/character position to a byte offset.
func offsetOf(s string, line, char int) int {
	off := 0
	for l := 0; l < line; l++ {
		i := strings.IndexByte(s[off:], '\n')
		if i < 0 {
			return len(s)
		}
		off += i + 1
	}
	if off+char > len(s) {
		return len(s)
	}
	return off + char
}

func makeClearDiagnosticNotification(uri string) []byte {
	diag := map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/publishDiagnostics",
		"params": map[string]any{
			"uri":         uri,
			"diagnostics": []map[string]any{},
		},
	}
	b, _ := json.Marshal(diag)
	return b
}

func (s *state) rewriteClientToGopls(raw []byte) ([]byte, error) {
	var m rpcMsg
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Method == "" || m.Params == nil {
		return raw, nil
	}

	switch m.Method {
	case "initialize":
		// pass through (workspaceFolders etc), but remember the id so we can strip gopls commands
		// from the initialize response. Otherwise the Go extension may already have registered
		// those commands, and vscode-languageclient will fail with "command already exists".
		s.mu.Lock()
		s.initializeID = m.ID
		s.mu.Unlock()
		return raw, nil

	case "textDocument/didOpen":
		var p struct {
			TextDocument struct {
				URI        string `json:"uri"`
				LanguageID string `json:"languageId"`
				Version    int    `json:"version"`
				Text       string `json:"text"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}

		goSrc, sm, err := safeCompile(uriToPath(p.TextDocument.URI), []byte(p.TextDocument.Text))
		if err != nil {
			goSrc = []byte("package " + packageNameOf(p.TextDocument.Text) + "\n")
			sm = nil
			s.setGSXDiagnostics(p.TextDocument.URI, gsxDiagnostic(err))
		} else {
			s.setGSXDiagnostics(p.TextDocument.URI, nil)
		}

		d := &docState{
			gsxURI:    p.TextDocument.URI,
			goURI:     gsxToGoURI(p.TextDocument.URI),
			gsxText:   p.TextDocument.Text,
			goText:    string(goSrc),
			sourceMap: sm,
		}
		s.upsertDoc(d)

		// Rewrite to go doc open.
		p.TextDocument.URI = d.goURI
		p.TextDocument.LanguageID = "go"
		p.TextDocument.Text = d.goText
		b, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)

	case "textDocument/didChange":
		var p struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}
		if len(p.ContentChanges) == 0 {
			return raw, nil
		}
		// VS Code usually sends full sync for these setups; if not, we still treat first change as full text.
		newText := p.ContentChanges[len(p.ContentChanges)-1].Text

		goSrc, sm, err := safeCompile(uriToPath(p.TextDocument.URI), []byte(newText))
		if err != nil {
			s.setGSXDiagnostics(p.TextDocument.URI, gsxDiagnostic(err))
			d := s.getDoc(p.TextDocument.URI)
			if d == nil {
				return raw, nil
			}
			p.TextDocument.URI = d.goURI
			p.ContentChanges = []struct {
				Text string `json:"text"`
			}{{Text: d.goText}}
			b, _ := json.Marshal(p)
			m.Params = (*json.RawMessage)(&b)
			return json.Marshal(m)
		}
		s.setGSXDiagnostics(p.TextDocument.URI, nil)

		d := s.getDoc(p.TextDocument.URI)
		if d == nil {
			d = &docState{gsxURI: p.TextDocument.URI, goURI: gsxToGoURI(p.TextDocument.URI)}
		}
		d.gsxText = newText
		d.goText = string(goSrc)
		d.sourceMap = sm
		s.upsertDoc(d)

		p.TextDocument.URI = d.goURI
		p.ContentChanges = []struct {
			Text string `json:"text"`
		}{{Text: d.goText}}
		b, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)

	case "textDocument/didClose":
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}
		gsxURI := p.TextDocument.URI
		d := s.getDoc(gsxURI)
		if d != nil {
			p.TextDocument.URI = d.goURI
		} else {
			p.TextDocument.URI = gsxToGoURI(gsxURI)
		}
		s.deleteDoc(gsxURI)
		b, _ := json.Marshal(p)
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)

	case "textDocument/formatting":
		// gopls cannot format a .gsx buffer, and the generated Go view is not
		// what the user is editing. Answer from the formatter directly.
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}
		s.setPendingResponse(formattingResponse(m.ID, s.getDoc(p.TextDocument.URI)))
		// Swallow the request: gopls must not see it.
		return nil, nil

	case "textDocument/completion":
		// Inside markup, GSX knows the answer and gopls does not: the generated
		// Go view has no notion of a tag name or an attribute.
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}
		d := s.getDoc(p.TextDocument.URI)
		if d == nil {
			return raw, nil
		}

		off := offsetOf(d.gsxText, p.Position.Line, p.Position.Character)
		if ctx := completionAt(d.gsxText, off); ctx.kind != completeNone {
			s.setPendingResponse(completionResponse(m.ID, completionsFor(ctx, d.gsxText)))
			return nil, nil
		}
		// Ordinary Go: let gopls answer, at the mapped position.
		gl, gc, ok := mapPositionSrcToGo(d, p.Position.Line, p.Position.Character)
		if !ok {
			s.setPendingResponse(nullResponse(m.ID))
			return nil, nil
		}
		p.TextDocument.URI = d.goURI
		p.Position.Line, p.Position.Character = gl, gc
		b, _ := json.Marshal(p)
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)

	case "textDocument/definition", "textDocument/hover", "textDocument/signatureHelp":
		// Rewrite position-bearing requests.
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position struct {
				Line      int `json:"line"`
				Character int `json:"character"`
			} `json:"position"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !isGSXURI(p.TextDocument.URI) {
			return raw, nil
		}
		d := s.getDoc(p.TextDocument.URI)
		if d == nil {
			s.setPendingResponse(nullResponse(m.ID))
			return nil, nil
		}
		if m.Method == "textDocument/definition" {
			if gl, gc, ok := mapDefinitionInsideBraces(d, p.Position.Line, p.Position.Character); ok {
				p.TextDocument.URI = d.goURI
				p.Position.Line = gl
				p.Position.Character = gc
				b, _ := json.Marshal(p)
				m.Params = (*json.RawMessage)(&b)
				return json.Marshal(m)
			}
		}
		gl, gc, ok := mapPositionSrcToGo(d, p.Position.Line, p.Position.Character)
		if !ok {
			// The position does not exist in the generated view — normally
			// because the file currently has a parse error, so there is no
			// source map. Answer with no result.
			//
			// Forwarding instead would send gopls a .gsx URI it has never been
			// told about, and it replies with an error that the editor shows as
			// "Request textDocument/hover failed" on every hover.
			s.setPendingResponse(nullResponse(m.ID))
			return nil, nil
		}
		p.TextDocument.URI = d.goURI
		p.Position.Line = gl
		p.Position.Character = gc
		b, _ := json.Marshal(p)
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)
	}

	return raw, nil
}

func (s *state) rewriteGoplsToClient(raw []byte) ([]byte, error) {
	var m rpcMsg
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	// If this is the initialize response, strip gopls command registrations to avoid collisions
	// with the Go extension, and force full document sync.
	if m.Result != nil {
		s.mu.Lock()
		initID := s.initializeID
		s.mu.Unlock()
		if initID != nil && idsEqual(m.ID, initID) {
			var resp map[string]any
			if err := json.Unmarshal(*m.Result, &resp); err == nil {
				stripCapabilities(resp)
				forceFullSync(resp)
				b, _ := json.Marshal(resp)
				rm := json.RawMessage(b)
				m.Result = &rm
				return json.Marshal(m)
			}
		}
	}

	// Notifications from gopls we care about.
	if m.Method == "textDocument/publishDiagnostics" && m.Params != nil {
		var p struct {
			URI         string `json:"uri"`
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
				Severity int    `json:"severity,omitempty"`
				Source   string `json:"source,omitempty"`
				Message  string `json:"message,omitempty"`
			} `json:"diagnostics"`
		}
		if err := json.Unmarshal(*m.Params, &p); err != nil {
			return nil, err
		}
		if !strings.HasSuffix(p.URI, ".gsx.go") {
			return raw, nil
		}
		gsxURI := goToGSXURI(p.URI)
		d := s.getDoc(gsxURI)
		if d == nil {
			// Best-effort: just rewrite the URI, still merging GSX's own.
			p.URI = gsxURI
			return mergedDiagnostics(s, p.URI, p.Diagnostics), nil
		}
		p.URI = gsxURI
		for i := range p.Diagnostics {
			st := p.Diagnostics[i].Range.Start
			en := p.Diagnostics[i].Range.End
			sl, sc, okS := mapPositionGoToSrc(d, st.Line, st.Character)
			el, ec, okE := mapPositionGoToSrc(d, en.Line, en.Character)
			if okS {
				p.Diagnostics[i].Range.Start.Line = sl
				p.Diagnostics[i].Range.Start.Character = sc
			}
			if okE {
				p.Diagnostics[i].Range.End.Line = el
				p.Diagnostics[i].Range.End.Character = ec
			} else if okS {
				// fall back to a zero-length range at start
				p.Diagnostics[i].Range.End.Line = sl
				p.Diagnostics[i].Range.End.Character = sc
			}
			if p.Diagnostics[i].Source == "" {
				p.Diagnostics[i].Source = "gopls"
			}
		}
		// publishDiagnostics replaces the whole set for a URI, so gopls's list
		// has to carry GSX's own diagnostics along with it. Otherwise a parse
		// error is published and then immediately wiped by whatever gopls says
		// about the same file a moment later.
		return mergedDiagnostics(s, p.URI, p.Diagnostics), nil
	}

	// Responses: best-effort rewrite any locations that point at *_gsx.go back to .gsx.
	// We only cover common shapes we expect from definition requests.
	if m.Result != nil {
		var v any
		if err := json.Unmarshal(*m.Result, &v); err != nil {
			return raw, nil
		}
		changed := rewriteResultLocationsInPlace(s, v)
		if changed {
			b, _ := json.Marshal(v)
			rm := json.RawMessage(b)
			m.Result = &rm
			return json.Marshal(m)
		}
	}
	return raw, nil
}

func idsEqual(a, b any) bool {
	// JSON-RPC ids can be string/number/null. Compare stringified canonical values.
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}

func stripCapabilities(resp map[string]any) map[string]any {
	removed := map[string]any{}
	// initialize result shape: { capabilities: { ... } }
	caps, _ := resp["capabilities"].(map[string]any)
	if caps == nil {
		return removed
	}

	// 1) gopls commands collide with the Go extension. Remove entirely.
	if _, ok := caps["executeCommandProvider"]; ok {
		delete(caps, "executeCommandProvider")
		removed["executeCommandProvider"] = true
	}

	// 2) Inlay hints are requested by Cursor/VS Code and gopls will error for *.gsx URIs unless
	// we fully proxy that method. Disable for now to keep logs clean.
	if _, ok := caps["inlayHintProvider"]; ok {
		delete(caps, "inlayHintProvider")
		removed["inlayHintProvider"] = true
	}

	// 2b) Code actions are also requested for *.gsx URIs; disable until we proxy CodeAction.
	if _, ok := caps["codeActionProvider"]; ok {
		delete(caps, "codeActionProvider")
		removed["codeActionProvider"] = true
	}

	// 3) Semantic tokens are also URI-sensitive and not yet proxied. Disable to avoid spurious errors.
	if _, ok := caps["semanticTokensProvider"]; ok {
		delete(caps, "semanticTokensProvider")
		removed["semanticTokensProvider"] = true
	}

	// GSX answers these itself, so they are advertised regardless of what gopls
	// reported for the Go view.
	caps["documentFormattingProvider"] = true
	caps["completionProvider"] = map[string]any{
		// `<` and `/` open tag completion, a space inside a start tag opens
		// attribute completion.
		"triggerCharacters": []string{"<", "/", " ", "."},
	}

	resp["capabilities"] = caps
	return removed
}

// forceFullSync overrides textDocumentSync to require full document content on every change,
// which is what our proxy expects.
func forceFullSync(resp map[string]any) {
	caps, _ := resp["capabilities"].(map[string]any)
	if caps == nil {
		return
	}
	caps["textDocumentSync"] = map[string]any{
		"openClose": true,
		"change":    1, // TextDocumentSyncKind.Full
	}
}

func rewriteResultLocationsInPlace(st *state, v any) bool {
	changed := false

	// Location: { uri, range: {start,end} }
	var rewriteLocation func(loc map[string]any) bool
	rewriteLocation = func(loc map[string]any) bool {
		uriV, _ := loc["uri"].(string)
		if !strings.HasSuffix(uriV, ".gsx.go") {
			return false
		}
		gsxURI := goToGSXURI(uriV)
		d := st.getDoc(gsxURI)
		loc["uri"] = gsxURI
		changedLoc := true
		rng, _ := loc["range"].(map[string]any)
		if d == nil || rng == nil {
			return changedLoc
		}
		mapPos := func(k string) {
			p, _ := rng[k].(map[string]any)
			if p == nil {
				return
			}
			ln, _ := p["line"].(float64)
			ch, _ := p["character"].(float64)
			sl, sc, ok := mapPositionGoToSrc(d, int(ln), int(ch))
			if !ok {
				return
			}
			p["line"] = sl
			p["character"] = sc
		}
		mapPos("start")
		mapPos("end")
		return changedLoc
	}

	switch t := v.(type) {
	case []any:
		for i := range t {
			if rewriteResultLocationsInPlace(st, t[i]) {
				changed = true
			}
		}
	case map[string]any:
		// direct location?
		if _, ok := t["uri"]; ok && t["range"] != nil {
			if rewriteLocation(t) {
				changed = true
			}
		}
		// or nested shapes
		for _, vv := range t {
			if rewriteResultLocationsInPlace(st, vv) {
				changed = true
			}
		}
	}
	return changed
}

func uriToPath(uri string) string {
	// VS Code sends file:// URIs. Keep it very lightweight for now.
	// We only need a stable "path" for error messages and ParseFile filename.
	u := strings.TrimPrefix(uri, "file://")
	// On Windows URIs might be file:///c:/...; strip an extra slash.
	if runtime.GOOS == "windows" && strings.HasPrefix(u, "/") && len(u) > 2 && u[2] == ':' {
		u = u[1:]
	}
	u = filepath.FromSlash(u)
	if u == "" {
		return "file.gsx"
	}
	// If it doesn't look like an absolute path, keep as-is.
	if !strings.HasPrefix(u, string(os.PathSeparator)) && !strings.Contains(u, ":") {
		return u
	}
	return u
}
