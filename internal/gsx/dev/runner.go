package dev

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Runner supervises the application process.
type Runner struct {
	// Command is the shell command that builds and starts the app.
	Command string
	// Dir is the working directory for the command.
	Dir string
	// Addr is where the app listens, used to wait for readiness after a start.
	Addr string
	// Out receives the app's stdout and stderr.
	Out io.Writer

	mu  sync.Mutex
	cmd *exec.Cmd
}

// Restart stops any running process and starts a fresh one, returning once the
// app is accepting connections.
//
// Waiting for readiness matters: telling the browser to reload before the new
// process is listening produces a connection error in the tab rather than the
// updated page.
func (r *Runner) Restart(ctx context.Context) error {
	r.Stop()

	cmd := exec.CommandContext(ctx, "sh", "-c", r.Command)
	cmd.Dir = r.Dir
	cmd.Stdout = r.Out
	cmd.Stderr = r.Out
	// Put the child in its own process group so the whole tree can be killed.
	// `go run` spawns the compiled binary as a grandchild; killing only the
	// direct child would orphan it still holding the port.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting %q: %w", r.Command, err)
	}

	r.mu.Lock()
	r.cmd = cmd
	r.mu.Unlock()

	// Reap the process so a crashed app does not linger as a zombie.
	go func() { _ = cmd.Wait() }()

	return r.waitReady(ctx)
}

// Stop terminates the app and its process group.
func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	r.cmd = nil
	r.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return
	}

	// Ask politely, then insist. A server given a moment to shut down releases
	// its listening socket, which avoids an "address already in use" on the
	// immediately following start.
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
}

// waitReady polls the app's address until it accepts a connection.
func (r *Runner) waitReady(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := net.DialTimeout("tcp", r.Addr, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("app did not start listening on %s within 30s", r.Addr)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
