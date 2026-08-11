package playground_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kilianc/gsx/internal/gsx/playground"
)

func run(t *testing.T, src string) playground.Result {
	t.Helper()
	res, err := playground.Run(context.Background(), src)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func TestRunRendersMarkup(t *testing.T) {
	res := run(t, `package main

func Page() Node {
	return (
		<main class="page">
			<h1>Hello</h1>
		</main>
	)
}
`)
	const want = `<main class="page"><h1>Hello</h1></main>`
	if res.HTML != want {
		t.Errorf("HTML = %q, want %q", res.HTML, want)
	}
	if !strings.Contains(res.Go, "html.Main(") {
		t.Errorf("Go output missing html.Main:\n%s", res.Go)
	}
}

// The point of interpreting rather than pattern-matching the tree: the parts of
// a page that are ordinary Go have to keep working.
func TestRunEvaluatesGo(t *testing.T) {
	res := run(t, `package main

import (
	"fmt"
	"strings"
)

type Product struct {
	Name  string
	Price float64
}

func Page() Node {
	items := []Product{{"Keyboard", 89.99}, {"Monitor", 329.50}}
	var rows []Node
	for _, p := range items {
		rows = append(rows, (
			<tr>
				<td>{strings.ToUpper(p.Name)}</td>
				<td class="price">{fmt.Sprintf("$%.2f", p.Price)}</td>
			</tr>
		))
	}
	return <tbody>{rows}</tbody>
}
`)
	for _, want := range []string{"KEYBOARD", "$89.99", "MONITOR", "$329.50"} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("HTML missing %q:\n%s", want, res.HTML)
		}
	}
}

func TestRunEscapesText(t *testing.T) {
	res := run(t, `package main

func Page() Node {
	return <p>{"<script>alert(1)</script>"}</p>
}
`)
	if strings.Contains(res.HTML, "<script>") {
		t.Errorf("text splice was not escaped: %s", res.HTML)
	}
	if !strings.Contains(res.HTML, "&lt;script&gt;") {
		t.Errorf("expected escaped output, got: %s", res.HTML)
	}
}

// The sandbox is the symbol table: a package that was never extracted cannot be
// resolved, so there is no way to reach the host from interpreted code.
func TestSandboxDeniesDangerousImports(t *testing.T) {
	for _, pkg := range []string{"os", "os/exec", "net/http", "syscall"} {
		t.Run(pkg, func(t *testing.T) {
			src := `package main

import "` + pkg + `"

func Page() Node {
	_ = ` + strings.Split(pkg, "/")[len(strings.Split(pkg, "/"))-1] + `.Getenv
	return <p>nope</p>
}
`
			_, err := playground.Run(context.Background(), src)
			if err == nil {
				t.Fatalf("importing %q was allowed; the sandbox leaks", pkg)
			}
			var perr *playground.Error
			if !errors.As(err, &perr) {
				t.Fatalf("error = %T, want *playground.Error", err)
			}
			if perr.Stage != playground.StageInterpret {
				t.Errorf("stage = %q, want %q", perr.Stage, playground.StageInterpret)
			}
		})
	}
}

func TestErrorStages(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want playground.Stage
	}{
		{
			name: "mismatched tag",
			src: `package main

func Page() Node { return <div>hi</span> }
`,
			want: playground.StageCompile,
		},
		{
			name: "undefined identifier",
			src: `package main

func Page() Node { return <p>{nope}</p> }
`,
			want: playground.StageInterpret,
		},
		{
			name: "missing entry point",
			src: `package main

func NotPage() Node { return <p>hi</p> }
`,
			want: playground.StageCall,
		},
		{
			name: "panic in user code",
			src: `package main

func Page() Node {
	var s []string
	return <p>{s[3]}</p>
}
`,
			want: playground.StageCall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := playground.Run(context.Background(), tt.src)
			if err == nil {
				t.Fatal("expected an error")
			}
			var perr *playground.Error
			if !errors.As(err, &perr) {
				t.Fatalf("error = %T, want *playground.Error", err)
			}
			if perr.Stage != tt.want {
				t.Errorf("stage = %q, want %q (%v)", perr.Stage, tt.want, err)
			}
		})
	}
}

// A compile error has no Go to show; anything later does, and the reader should
// keep seeing it while they fix the runtime problem.
func TestGoOutputSurvivesLaterFailures(t *testing.T) {
	res, err := playground.Run(context.Background(), `package main

func Page() Node { return <p>{nope}</p> }
`)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(res.Go, "html.P(") {
		t.Errorf("generated Go was not returned alongside the error:\n%s", res.Go)
	}
}

func TestCompileDoesNotRun(t *testing.T) {
	// Compile only translates, so an entry point that would panic is fine.
	out, err := playground.Compile(`package main

func Page() Node {
	var s []string
	return <p>{s[3]}</p>
}
`)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(out, "html.P(") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestNonMainPackage(t *testing.T) {
	res := run(t, `package ui

func Page() Node { return <p>ok</p> }
`)
	if res.HTML != "<p>ok</p>" {
		t.Errorf("HTML = %q", res.HTML)
	}
}
