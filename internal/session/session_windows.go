//go:build windows

package session

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StartAmbientWatcher is a no-op on Windows because bridge-managed sessions
// are restored from the durable print-mode session store.
func (m *Manager) StartAmbientWatcher(ctx context.Context) {}

// start creates a Windows Claude print-mode session. Claude Code's default mode
// expects an interactive TTY; on Windows the reliable bridge path is one
// non-interactive `claude -p --session-id <uuid>` process per user turn.
// Caller must hold m.mu.
func (m *Manager) start(sid, workDir string) (*Session, error) {
	claudeSessionID, err := newUUID()
	if err != nil {
		return nil, err
	}
	s := m.newWindowsSession(sid, workDir, claudeSessionID)
	m.sessions[sid] = s
	m.savePlatformSessionsLocked()
	slog.Info("windows session started", "id", sid, "cwd", workDir, "claude_session_id", claudeSessionID)
	return s, nil
}

func (m *Manager) newWindowsSession(sid, workDir, claudeSessionID string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	inputs := make(chan string, 16)

	s := &Session{
		ID:              sid,
		WorkDir:         workDir,
		ClaudeSessionID: claudeSessionID,
		cancel:          cancel,
		alive:           true,
		writeFunc:       enqueueWindowsInput(ctx, sid, inputs),
	}

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
	return s
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
		"--output-format", "stream-json",
		"--include-partial-messages",
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.onOutput(sid, "SENDER:System\nClaude turn failed: "+err.Error())
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.onOutput(sid, "SENDER:System\nClaude turn failed: "+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		m.onOutput(sid, "SENDER:System\nClaude turn failed: "+err.Error())
		return
	}

	stderrDone := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(stderr)
		stderrDone <- strings.TrimSpace(string(b))
	}()

	lastAssistant := ""
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 128*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		chunk, ok := claudeEventToOutput(line, &lastAssistant)
		if ok {
			m.onOutput(sid, chunk)
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		m.onOutput(sid, "SENDER:System\nClaude stream failed: "+err.Error())
	}

	err = cmd.Wait()
	stderrText := <-stderrDone
	if err != nil && ctx.Err() == nil {
		hint := err.Error()
		if stderrText != "" {
			hint = stderrText
		} else {
			hint = fmt.Sprintf("%s\ncommand: %s", hint, renderWindowsClaudeCommand(bin, args))
		}
		m.onOutput(sid, "SENDER:System\nClaude turn failed: "+hint)
	}
}

func claudeEventToOutput(line string, lastAssistant *string) (string, bool) {
	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "SENDER:Assistant\n" + line, true
	}

	eventType, _ := event["type"].(string)
	subtype, _ := event["subtype"].(string)
	switch {
	case eventType == "assistant" || eventType == "message":
		message := event
		if nested, ok := event["message"].(map[string]any); ok {
			message = nested
		}
		text := uniqueJoin(iterClaudeText(message["content"]))
		if text == "" {
			return "", false
		}
		if lastAssistant != nil && strings.HasPrefix(text, *lastAssistant) {
			delta := strings.TrimSpace(strings.TrimPrefix(text, *lastAssistant))
			*lastAssistant = text
			if delta == "" {
				return "", false
			}
			return "SENDER:Assistant\n" + delta, true
		}
		if lastAssistant != nil {
			*lastAssistant = text
		}
		return "SENDER:Assistant\n" + text, true
	case eventType == "result" || eventType == "final":
		text := uniqueJoin(iterClaudeText(firstPresent(event, "result", "content", "message")))
		if text == "" {
			return "", false
		}
		return "SENDER:Assistant\n" + text, true
	case strings.Contains(eventType, "tool") || strings.Contains(subtype, "tool"):
		tool := stringValue(firstPresent(event, "name", "tool_name", "toolName"))
		if tool == "" {
			tool = "tool"
		}
		body := uniqueJoin(iterClaudeText(firstPresent(event, "result", "tool_response", "content")))
		if body == "" {
			return "SENDER:Assistant/Tool\n" + tool, true
		}
		return "SENDER:Assistant/Tool\n" + tool + "\n" + body, true
	case eventType == "error" || eventType == "system" || subtype == "error" || subtype == "error_max_turns":
		text := uniqueJoin(iterClaudeText(firstPresent(event, "error", "message")))
		if text == "" {
			text = line
		}
		return "SENDER:System\n" + text, true
	default:
		return "", false
	}
}

func iterClaudeText(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return nil
		}
		return []string{text}
	case float64, bool:
		return []string{fmt.Sprint(v)}
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, iterClaudeText(item)...)
		}
		return parts
	case map[string]any:
		blockType, _ := v["type"].(string)
		if blockType == "text" || blockType == "output_text" {
			return iterClaudeText(v["text"])
		}
		keys := []string{"stdout", "stderr", "output", "error", "result", "message", "content", "text"}
		if blockType == "tool_result" || blockType == "tool_output" {
			keys = []string{"output", "stdout", "stderr", "error", "result", "content"}
		}
		var parts []string
		for _, key := range keys {
			parts = append(parts, iterClaudeText(v[key])...)
		}
		return parts
	default:
		return nil
	}
}

func uniqueJoin(parts []string) string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, "\n\n")
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
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
