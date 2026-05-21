//go:build windows

package session

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const windowsSessionsFile = "windows-sessions.json"

func (m *Manager) restorePlatformSessions() {
	path := m.platformSessionsPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var records []platformSessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		slog.Warn("restore windows sessions failed", "err", err)
		return
	}

	maxID := 0
	for _, record := range records {
		if record.ID == "" || record.WorkDir == "" || record.ClaudeSessionID == "" {
			continue
		}
		if n, err := strconv.Atoi(record.ID); err == nil && n > maxID {
			maxID = n
		}
		session := m.newWindowsSession(record.ID, record.WorkDir, record.ClaudeSessionID)
		m.sessions[record.ID] = session
		if record.Default {
			m.defaultSession = record.ID
		}
	}
	if maxID > m.nextID {
		m.nextID = maxID
	}
}

func (m *Manager) savePlatformSessionsLocked() {
	path := m.platformSessionsPath()
	if path == "" {
		return
	}

	records := make([]platformSessionRecord, 0, len(m.sessions))
	for id, s := range m.sessions {
		if s.ClaudeSessionID == "" {
			continue
		}
		records = append(records, platformSessionRecord{
			ID:              id,
			WorkDir:         s.WorkDir,
			ClaudeSessionID: s.ClaudeSessionID,
			Default:         id == m.defaultSession,
		})
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		slog.Warn("save windows sessions failed", "err", err)
		return
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		slog.Warn("save windows sessions failed", "err", err)
		return
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		slog.Warn("save windows sessions failed", "err", err)
	}
}

func (m *Manager) platformSessionsPath() string {
	if strings.TrimSpace(m.stateDir) == "" {
		return ""
	}
	return filepath.Join(m.stateDir, windowsSessionsFile)
}
