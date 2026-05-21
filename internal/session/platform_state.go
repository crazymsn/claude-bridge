package session

// platformSessionRecord is the durable subset of a bridge-managed session.
// Platform files decide whether and how these records are persisted.
type platformSessionRecord struct {
	ID              string `json:"id"`
	WorkDir         string `json:"work_dir"`
	ClaudeSessionID string `json:"claude_session_id"`
	Default         bool   `json:"default"`
}
