//go:build windows

package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Info is a session snapshot used by the list command.
type Info struct {
	ID          string
	WorkDir     string
	Interactive bool
	Running     bool
	Source      string
	PID         int
}

// Inspect enumerates durable Windows print-mode sessions. These sessions are
// resumed with their stable Claude session IDs when the bridge starts again.
func Inspect() ([]Info, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".claude-bridge", windowsSessionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var records []platformSessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}

	infos := make([]Info, 0, len(records))
	for _, record := range records {
		if record.ID == "" {
			continue
		}
		infos = append(infos, Info{
			ID:          record.ID,
			WorkDir:     record.WorkDir,
			Interactive: true,
			Running:     true,
			Source:      "windows-print",
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })
	return infos, nil
}
