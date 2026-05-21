//go:build windows

package session

// Info is a session snapshot used by the list command.
type Info struct {
	ID          string
	WorkDir     string
	Interactive bool
	Running     bool
	Source      string
	PID         int
}
