//go:build windows

package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StartAmbientWatcher is a no-op on Windows. Windows support currently covers
// sessions created by the bridge itself via /new; attaching to arbitrary
// already-running Claude terminals needs a future ConPTY backend.
func (m *Manager) StartAmbientWatcher(ctx context.Context) {}

// Inspect returns an empty list on Windows because bridge-created sessions are
// in-process runtime state, not discoverable through FIFO manifests.
func Inspect() ([]Info, error) {
	return nil, nil
}

// start launches Claude as a child process and wires stdin/stdout/stderr to the
// bridge. Caller must hold m.mu.
func (m *Manager) start(sid, workDir string) (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "claude")
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start claude: %w", err)
	}

	s := &Session{
		ID:      sid,
		WorkDir: workDir,
		proc:    cmd,
		writer:  stdin,
		cancel:  cancel,
		alive:   true,
	}
	m.sessions[sid] = s

	go m.forwardProcessOutput(ctx, sid, "Assistant", stdout)
	go m.forwardProcessOutput(ctx, sid, "System", stderr)
	go func() {
		err := cmd.Wait()
		if err != nil && ctx.Err() == nil {
			m.onOutput(sid, "SENDER:System\nsession ended: "+err.Error())
		} else {
			m.onOutput(sid, "SENDER:System\nsession ended")
		}
		m.cleanupSession(sid)
	}()

	slog.Info("windows session started", "id", sid, "cwd", workDir, "pid", cmd.Process.Pid)
	return s, nil
}

func (m *Manager) forwardProcessOutput(ctx context.Context, sid, sender string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var buf strings.Builder
	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text != "" {
			m.onOutput(sid, "SENDER:"+sender+"\n"+text)
			buf.Reset()
		}
	}

	ticker := time.NewTicker(1200 * time.Millisecond)
	defer ticker.Stop()
	lines := make(chan string, 32)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				flush()
				return
			}
			buf.WriteString(line)
			buf.WriteByte('\n')
			if buf.Len() > 3000 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func openPipeWriter(path string, timeout time.Duration) (io.WriteCloser, error) {
	return nil, fmt.Errorf("named pipes are not used on Windows")
}
