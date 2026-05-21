from __future__ import annotations

import json
import os
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any


def default_state_dir() -> Path:
    return Path(os.environ.get("CLAUDE_BRIDGE_STATE_DIR", Path.home() / ".claude-bridge"))


@dataclass
class SessionRecord:
    sid: str
    work_dir: str
    claude_session_id: str
    default: bool = False


class StateStore:
    def __init__(self, root: Path | None = None) -> None:
        self.root = root or default_state_dir()
        self.root.mkdir(parents=True, exist_ok=True)
        self.sessions_path = self.root / "python-sessions.json"
        self.ambient_user_path = self.root / "ambient-user.txt"

    @property
    def credentials_path(self) -> Path:
        return self.root / "credentials.json"

    @property
    def pid_path(self) -> Path:
        return self.root / "bridge.pid"

    @property
    def log_path(self) -> Path:
        return self.root / "bridge.log"

    def load_sessions(self) -> dict[str, SessionRecord]:
        try:
            raw = json.loads(self.sessions_path.read_text("utf-8"))
        except (FileNotFoundError, json.JSONDecodeError):
            return {}
        records: dict[str, SessionRecord] = {}
        for sid, item in raw.items():
            if isinstance(item, dict):
                try:
                    records[sid] = SessionRecord(**item)
                except TypeError:
                    continue
        return records

    def save_sessions(self, sessions: dict[str, SessionRecord]) -> None:
        payload: dict[str, Any] = {sid: asdict(record) for sid, record in sessions.items()}
        self.sessions_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", "utf-8")

    def load_target_user(self) -> str:
        try:
            return self.ambient_user_path.read_text("utf-8").strip()
        except FileNotFoundError:
            return ""

    def save_target_user(self, user_id: str) -> None:
        self.ambient_user_path.write_text(user_id.strip(), "utf-8")
