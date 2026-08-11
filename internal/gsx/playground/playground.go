// Package playground compiles and runs a single GSX source file in memory.
//
// It backs the browser playground, but nothing here depends on wasm: the same
// entry points run natively, which is what lets the behaviour be tested in
// ordinary CI instead of only in a browser. cmd/gsx-wasm is a thin shim over
// this package.
//
// Running the compiler's output means executing the reader's code. Rather than
// building it — which needs a Go toolchain, so a server, so a sandbox — the
// output is interpreted, and the interpreter is given only the packages in
// internal/gsx/playground/symbols. Interpreted code cannot open a file or a
// socket because os and net are not in that table at all.
package playground

//go:generate go run ./gen

import (
	"context"
	"fmt"
	goparser "go/parser"
	gotoken "go/token"
	"strings"

	"maragu.dev/gomponents"

	"github.com/kilianc/gsx/internal/gsx/playground/symbols"
	"github.com/kilianc/gsx/pkg/gsx"

	"github.com/traefik/yaegi/interp"
)

// EntryPoint is the function the playground renders.
//
// Markup usually takes arguments, and a playground has nobody to supply them,
// so the contract is a nullary function that closes over its own example data.
const EntryPoint = "Page"

// Stage is the step of the pipeline that failed, so a caller can show the
// error against the pane it belongs to.
type Stage string

const (
	StageCompile   Stage = "compile"   // .gsx did not translate to Go
	StageInterpret Stage = "interpret" // the Go did not load
	StageCall      Stage = "call"      // EntryPoint was missing, or panicked
	StageRender    Stage = "render"    // the Node tree would not serialise
)

// Error is a failure at a known stage of the pipeline.
type Error struct {
	Stage Stage
	Err   error
}

func (e *Error) Error() string { return string(e.Stage) + ": " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Result is the output of a successful run.
type Result struct {
	// Go is the generated source. It is filled in whenever compilation
	// succeeded, including when a later stage failed, so the reader can still
	// see what their markup became.
	Go string
	// HTML is the rendered markup.
	HTML string
}

// Compile translates GSX source to Go source. It neither loads nor runs the
// result, so it cannot execute anything the caller passes in.
func Compile(src string) (string, error) {
	out, err := gsx.CompileFile("page.gsx", []byte(src))
	if err != nil {
		return "", &Error{Stage: StageCompile, Err: err}
	}
	return string(out), nil
}

// Run compiles src, interprets it, calls EntryPoint and renders the result.
//
// ctx bounds the interpreter, but only where the Go runtime can preempt it:
// under wasm the scheduler is cooperative, so a tight loop in interpreted code
// will not observe cancellation. Callers that run untrusted input must be able
// to kill the whole instance — in the browser that means a worker the page can
// terminate, which is why cmd/gsx-wasm is loaded into one.
func Run(ctx context.Context, src string) (Result, error) {
	goSrc, err := Compile(src)
	if err != nil {
		return Result{}, err
	}
	res := Result{Go: goSrc}

	i := interp.New(interp.Options{})
	// Only these packages exist as far as interpreted code is concerned.
	if err := i.Use(symbols.Symbols); err != nil {
		return res, &Error{Stage: StageInterpret, Err: err}
	}

	if _, err := i.EvalWithContext(ctx, goSrc); err != nil {
		return res, &Error{Stage: StageInterpret, Err: err}
	}

	node, err := call(ctx, i, goSrc)
	if err != nil {
		return res, err
	}

	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		return res, &Error{Stage: StageRender, Err: err}
	}
	res.HTML = sb.String()
	return res, nil
}

// call evaluates EntryPoint and returns the Node it produced.
//
// The call is made inside the interpreter rather than through a Go function
// value so that it runs under ctx: reflect-calling it from here would escape
// cancellation entirely.
func call(ctx context.Context, i *interp.Interpreter, goSrc string) (n gomponents.Node, err error) {
	// Interpreted code is the reader's, so it can panic on its own — an index
	// out of range should surface as a message, not take the process with it.
	defer func() {
		if r := recover(); r != nil {
			n, err = nil, &Error{Stage: StageCall, Err: fmt.Errorf("panic: %v", r)}
		}
	}()

	expr := EntryPoint + "()"
	if pkg := packageName(goSrc); pkg != "" && pkg != "main" {
		// A non-main package is still registered, just under its own name.
		expr = pkg + "." + expr
	}

	v, err := i.EvalWithContext(ctx, expr)
	if err != nil {
		return nil, &Error{Stage: StageCall, Err: fmt.Errorf(
			"could not call %s: %w\n\nthe playground renders `func %s() Node`",
			expr, err, EntryPoint)}
	}

	node, ok := v.Interface().(gomponents.Node)
	if !ok {
		return nil, &Error{Stage: StageCall, Err: fmt.Errorf(
			"%s returned %T, want Node", expr, v.Interface())}
	}
	if node == nil {
		return nil, &Error{Stage: StageCall, Err: fmt.Errorf("%s returned nil", expr)}
	}
	return node, nil
}

// packageName reads the package clause from generated Go, so the entry point
// can be addressed whichever package the reader declared.
func packageName(goSrc string) string {
	f, err := goparser.ParseFile(gotoken.NewFileSet(), "page.go", goSrc, goparser.PackageClauseOnly)
	if err != nil || f.Name == nil {
		return ""
	}
	return f.Name.Name
}
