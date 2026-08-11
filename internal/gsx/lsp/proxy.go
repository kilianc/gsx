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
			// Capture any pending client notification to send back.
			if diag := st.popPendingDiagnostic(); diag != nil {
				_ = WriteMessage(stdout, diag)
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

	// Pending diagnostic notification to send to the client (e.g. compile errors).
	pendingDiag []byte
}

func newState() *state {
	return &state{
		docs: map[string]*docState{},
	}
}

func (s *state) setPendingDiagnostic(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingDiag = msg
}

func (s *state) popPendingDiagnostic() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.pendingDiag
	s.pendingDiag = nil
	return d
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

// makeCompileDiagnostic builds a diagnostic for a GSX compile failure.
//
// A *parse.Error knows the exact offset that failed, so the squiggle lands on
// the offending character instead of at the top of the file. Its rendered form
// repeats the path and a source snippet, which the editor already shows, so the
// message is reduced to the bare text.
func makeCompileDiagnostic(uri string, err error) []byte {
	line, char := 0, 0
	msg := err.Error()

	var pe *parse.Error
	if errors.As(err, &pe) {
		l, c := pe.Position()
		// LSP positions are 0-based; parse.Error reports 1-based.
		line, char = l-1, c-1
		msg = pe.Msg
	}

	return makeDiagnosticNotification(uri, line, char, "GSX: "+msg)
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

		goSrc, sm, err := compile.CompileFileForLSP(uriToPath(p.TextDocument.URI), []byte(p.TextDocument.Text))
		if err != nil {
			goSrc = []byte("package p\n")
			sm = nil
			s.setPendingDiagnostic(makeCompileDiagnostic(p.TextDocument.URI, err))
		} else {
			s.setPendingDiagnostic(makeClearDiagnosticNotification(p.TextDocument.URI))
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

		goSrc, sm, err := compile.CompileFileForLSP(uriToPath(p.TextDocument.URI), []byte(newText))
		if err != nil {
			s.setPendingDiagnostic(makeCompileDiagnostic(p.TextDocument.URI, err))
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
		s.setPendingDiagnostic(makeClearDiagnosticNotification(p.TextDocument.URI))

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

	case "textDocument/completion", "textDocument/definition", "textDocument/hover", "textDocument/signatureHelp":
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
			return raw, nil
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
			// Can't map; let gopls decide (or return empty).
			return raw, nil
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
			// Best-effort: just rewrite URI.
			p.URI = gsxURI
			b, _ := json.Marshal(p)
			m.Params = (*json.RawMessage)(&b)
			return json.Marshal(m)
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
		b, _ := json.Marshal(p)
		m.Params = (*json.RawMessage)(&b)
		return json.Marshal(m)
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
