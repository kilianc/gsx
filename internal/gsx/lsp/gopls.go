package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func findGopls() (string, error) {
	name := "gopls"
	if runtime.GOOS == "windows" {
		name = "gopls.exe"
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	} else if !errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("unexpected error looking for gopls: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unexpected error looking for gopls: %w", err)
	}
	locations := []string{
		filepath.Join(home, "go", "bin", name),
		filepath.Join(home, ".local", "bin", name),
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc, nil
		}
	}
	return "", fmt.Errorf("cannot find gopls (searched PATH and common locations). Install with `go install golang.org/x/tools/gopls@latest`")
}

type goplsProc struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func startGopls(args []string) (*goplsProc, error) {
	goplsPath, err := findGopls()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(goplsPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	return &goplsProc{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

// GracefulShutdown sends an LSP shutdown request followed by an exit notification,
// then waits for the process to exit. Falls back to kill after a timeout.
func (p *goplsProc) GracefulShutdown() error {
	if p == nil {
		return nil
	}

	shutdownReq, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      99999,
		"method":  "shutdown",
	})
	exitNotif, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "exit",
	})

	_ = WriteMessage(p.stdin, shutdownReq)
	_ = WriteMessage(p.stdin, exitNotif)
	_ = p.stdin.Close()

	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	}

	_ = p.stdout.Close()
	return nil
}

func (p *goplsProc) Close() error {
	if p == nil {
		return nil
	}
	_ = p.stdin.Close()
	_ = p.stdout.Close()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return nil
}
