//go:build js && wasm

// Command gsx-wasm exposes the playground to JavaScript.
//
// It is built for the browser and loaded by the documentation site:
//
//	GOOS=js GOARCH=wasm go build -ldflags=-s -w -o docs/static/gsx.wasm ./cmd/gsx-wasm
//
// The build tag keeps it out of ordinary `go build ./...`, so CI compiles it
// with an explicit wasm step instead.
//
// Two functions are installed on the global object:
//
//	gsxCompile(src)   -> {go}          | {error, stage}
//	gsxRun(src)       -> {go, html}    | {error, stage, go?}
//	gsxHighlight(src) -> string
//
// All three are synchronous. That is only reasonable because this runs inside
// a worker: a page that called them directly would freeze on a runaway loop,
// where a worker can simply be terminated.
package main

import (
	"context"
	"errors"
	"syscall/js"

	"github.com/kilianc/gsx/internal/gsx/highlight"
	"github.com/kilianc/gsx/internal/gsx/playground"
)

func main() {
	js.Global().Set("gsxCompile", js.FuncOf(compile))
	js.Global().Set("gsxRun", js.FuncOf(run))
	js.Global().Set("gsxHighlight", js.FuncOf(highlightSrc))

	// The worker cannot post its ready message until the exports above exist.
	if ready := js.Global().Get("gsxReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}

	// Keep the instance alive; the exported functions are called from JS.
	select {}
}

func compile(_ js.Value, args []js.Value) any {
	src, err := source(args)
	if err != nil {
		return fail(err)
	}
	out, err := playground.Compile(src)
	if err != nil {
		return fail(err)
	}
	res := map[string]any{}
	withGo(res, out)
	return res
}

// withGo attaches the generated source both raw and pre-highlighted, so the
// page never has to make a second call just to colour what it already has.
func withGo(out map[string]any, goSrc string) {
	if goSrc == "" {
		return
	}
	out["go"] = goSrc
	out["go_html"] = highlight.HTML(goSrc)
}

func run(_ js.Value, args []js.Value) any {
	src, err := source(args)
	if err != nil {
		return fail(err)
	}

	res, err := playground.Run(context.Background(), src)
	out := fail(err)
	if err == nil {
		out = map[string]any{}
	}
	// The generated Go is worth showing even when a later stage failed.
	withGo(out, res.Go)
	if err == nil {
		out["html"] = res.HTML
	}
	return out
}

// highlightSrc tokenises GSX (or the Go the compiler emits) into classed
// spans. The site's own highlighter is already linked in, so the playground
// gets the same colouring as every code sample on the site without shipping a
// JavaScript one.
func highlightSrc(_ js.Value, args []js.Value) any {
	src, err := source(args)
	if err != nil {
		return ""
	}
	return highlight.HTML(src)
}

func source(args []js.Value) (string, error) {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return "", errors.New("expected a source string")
	}
	return args[0].String(), nil
}

// fail shapes an error for JS, tagging it with the stage so the page can show
// it against the pane it belongs to.
func fail(err error) map[string]any {
	if err == nil {
		return map[string]any{}
	}
	out := map[string]any{"error": err.Error(), "stage": string(playground.StageCompile)}

	var perr *playground.Error
	if errors.As(err, &perr) {
		// Unwrap so the message is the compiler's own, carets and all, rather
		// than one prefixed with a stage the UI is already showing.
		out["error"] = perr.Err.Error()
		out["stage"] = string(perr.Stage)
		if d := diagnostics(perr.Diagnostics); d != nil {
			out["diagnostics"] = d
		}
	}
	return out
}

// diagnostics converts to the plain values syscall/js can hand to JavaScript.
func diagnostics(ds []playground.Diagnostic) []any {
	if len(ds) == 0 {
		return nil
	}
	out := make([]any, 0, len(ds))
	for _, d := range ds {
		out = append(out, map[string]any{
			"line":     d.Line,
			"col":      d.Col,
			"message":  d.Message,
			"severity": d.Severity,
		})
	}
	return out
}
