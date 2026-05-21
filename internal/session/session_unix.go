//go:build !windows

package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"log/slog"

	"github.com/crazymsn/claude-bridge/internal/app"
)

// start launches a claude process in Terminal and a named-pipe reader.
// Caller must hold m.mu.
func (m *Manager) start(sid, workDir string) (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pipePath := fmt.Sprintf("/tmp/claude-hook-%s.pipe", sid)
	inputPipePath := fmt.Sprintf("/tmp/claude-input-%s.pipe", sid)
	os.Remove(pipePath)
	os.Remove(inputPipePath)
	if err := syscall.Mkfifo(pipePath, 0600); err != nil {
		cancel()
		return nil, fmt.Errorf("mkfifo: %w", err)
	}
	if err := syscall.Mkfifo(inputPipePath, 0600); err != nil {
		cancel()
		_ = os.Remove(pipePath)
		return nil, fmt.Errorf("mkfifo input: %w", err)
	}

	if err := launchClaudeInTerminal(workDir, sid, pipePath, inputPipePath); err != nil {
		cancel()
		_ = os.Remove(pipePath)
		_ = os.Remove(inputPipePath)
		return nil, fmt.Errorf("open terminal: %w", err)
	}

	s := &Session{
		ID:      sid,
		WorkDir: workDir,
		InPipe:  inputPipePath,
		cancel:  cancel,
	}
	m.sessions[sid] = s

	go func() {
		m.readPipe(ctx, pipePath, sid)
		m.cleanupSession(sid)
	}()

	slog.Info("session started", "id", sid, "cwd", workDir)
	return s, nil
}

func launchClaudeInTerminal(workDir, sid, outputPipePath, inputPipePath string) error {
	launchDir := "/tmp/claude-bridge-launch"
	if err := os.MkdirAll(launchDir, 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(registryDir, 0700); err != nil {
		return err
	}

	muxPath, err := app.AssetPath("scripts", "claude_bridge_mux.py")
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(registryDir, sid+".json")
	scriptPath := filepath.Join(launchDir, sid+".command")
	scriptBody := fmt.Sprintf(`#!/bin/zsh
set -euo pipefail
cleanup() {
  rm -f %s %s %s
}
trap cleanup EXIT INT TERM
cd %s
export CLAUDE_SESSION_ID=%s
export CLAUDE_HOOK_PIPE=%s
cat > %s <<EOF
{"id":"%s","pid":$$,"work_dir":%s,"input_pipe":%s,"output_pipe":%s}
EOF
printf '%%s\n' %s
python3 %s %s -- claude
`, shellQuote(inputPipePath), shellQuote(outputPipePath), shellQuote(manifestPath),
		shellQuote(workDir),
		shellQuote(sid),
		shellQuote(outputPipePath),
		shellQuote(manifestPath),
		sid,
		strconv.Quote(workDir),
		strconv.Quote(inputPipePath),
		strconv.Quote(outputPipePath),
		shellQuote("[claude-bridge] session #"+sid+" ready"),
		shellQuote(muxPath),
		shellQuote(inputPipePath),
	)
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0700); err != nil {
		return err
	}

	cmd := exec.Command("open", "-a", "Terminal", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openPipeWriter(path string, timeout time.Duration) (io.WriteCloser, error) {
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0600)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.ENXIO) && !errors.Is(err, syscall.ENOENT) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
