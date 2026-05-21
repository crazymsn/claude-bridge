//go:build windows

package session

import (
	"context"
	"crypto/rand"
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

// start creates a Windows Claude print-mode session. Claude Code's default mode
// expects an interactive TTY; on Windows the reliable bridge path is one
// non-interactive `claude -p --session-id <uuid>` process per user turn.
// Caller must hold m.mu.
func (m *Manager) start(sid, workDir string) (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())
	claudeSessionID, err := newUUID()
	if err != nil {
		cancel()
		return nil, err
	}
	inputs := make(chan string, 16)

	s := &Session{
		ID:        sid,
		WorkDir:   workDir,
		cancel:    cancel,
		alive:     true,
		writeFunc: enqueueWindowsInput(ctx, sid, inputs),
	}
	m.sessions[sid] = s

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case text := <-inputs:
				m.runWindowsClaudeTurn(ctx, sid, workDir, claudeSessionID, text)
			}
		}
	}()

	slog.Info("windows session started", "id", sid, "cwd", workDir, "claude_session_id", claudeSessionID)
	return s, nil
}

func enqueueWindowsInput(ctx context.Context, sid string, inputs chan<- string) func(string) error {
	return func(text string) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("session #%s has ended", sid)
		case inputs <- text:
			return nil
		default:
			return fmt.Errorf("session #%s input queue is full; wait for the current turn to finish", sid)
		}
	}
}

func (m *Manager) runWindowsClaudeTurn(ctx context.Context, sid, workDir, claudeSessionID, text string) {
	args := []string{
		"-p",
		"--session-id", claudeSessionID,
	}
	if mode := strings.TrimSpace(os.Getenv("CLAUDE_BRIDGE_PERMISSION_MODE")); mode != "" {
		args = append(args, "--permission-mode", mode)
	}
	args = append(args, "--", text)

	bin := strings.TrimSpace(os.Getenv("CLAUDE_BRIDGE_CLAUDE_BIN"))
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	body := strings.TrimSpace(string(output))
	if body != "" {
		m.onOutput(sid, "SENDER:Assistant\n"+body)
	}
	if err != nil && ctx.Err() == nil {
		hint := err.Error()
		if body == "" {
			hint = fmt.Sprintf("%s\ncommand: %s", hint, renderWindowsClaudeCommand(bin, args))
		}
		m.onOutput(sid, "SENDER:System\nClaude turn failed: "+hint)
	}
}

func openPipeWriter(path string, timeout time.Duration) (io.WriteCloser, error) {
	return nil, fmt.Errorf("named pipes are not used on Windows")
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func renderWindowsClaudeCommand(bin string, args []string) string {
	rendered := []string{bin}
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\r\n\"") {
			rendered = append(rendered, strconvQuote(arg))
			continue
		}
		rendered = append(rendered, arg)
	}
	return strings.Join(rendered, " ")
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
