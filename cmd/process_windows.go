//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func bridgeSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}

func configureBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func terminatePID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}

func isPIDAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH").Output()
	if err != nil {
		return false
	}
	text := strings.TrimSpace(string(out))
	return text != "" && !strings.Contains(text, "No tasks are running") && strings.Contains(text, fmt.Sprintf("%d", pid))
}

func runtimeCleanupPaths() []string {
	tmp := os.TempDir()
	var paths []string
	for _, dir := range []string{
		"claude-bridge-elicit",
		"claude-bridge-perm",
		"claude-bridge-launch",
		"claude-bridge-hooks",
		"claude-bridge-sessions",
		"claude-bridge-interact",
		"claude-bridge-state",
	} {
		paths = append(paths, filepath.Join(tmp, dir))
	}
	return paths
}
