//go:build !windows

package session

// macOS/Linux interactive sessions are discovered from live FIFO manifests.
// There is no separate durable session list to restore here.
func (m *Manager) restorePlatformSessions() {}

func (m *Manager) savePlatformSessionsLocked() {}
