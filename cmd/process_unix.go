//go:build !windows

package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

func bridgeSignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

func configureBackgroundCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func terminatePID(pid int) error {
	err := syscall.Kill(pid, syscall.SIGTERM)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func isPIDAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func runtimeCleanupPaths() []string {
	var paths []string
	for _, dir := range []string{
		"/tmp/claude-bridge-elicit",
		"/tmp/claude-bridge-perm",
		"/tmp/claude-bridge-launch",
		"/tmp/claude-bridge-hooks",
		"/tmp/claude-bridge-sessions",
	} {
		paths = append(paths, dir)
	}
	for _, pattern := range []string{
		"/tmp/claude-bridge-control.pipe",
		"/tmp/claude-hook-*.pipe",
		"/tmp/claude-input-*.pipe",
	} {
		matches, _ := filepath.Glob(pattern)
		paths = append(paths, matches...)
	}
	return paths
}
