package gsx

import "github.com/kilianc/gsx/internal/gsx/compile"

// CompileFile compiles a Go-first .gsx source (a Go file with embedded `<tag>` expressions)
// into a gofmt'd Go source file.
//
// The result is suitable for writing to "<path>.go" (i.e. "*.gsx.go") and checking in.
func CompileFile(path string, src []byte) ([]byte, error) {
	return compile.CompileFile(path, src)
}
