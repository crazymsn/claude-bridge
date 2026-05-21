from __future__ import annotations

import asyncio
import os
import uuid
from pathlib import Path
from typing import Awaitable, Callable

from .claude_stream import ClaudeStreamRunner
from .state import SessionRecord, StateStore

SendFunc = Callable[[str], Awaitable[None]]


class SessionManager:
    def __init__(self, store: StateStore, send: SendFunc) -> None:
        self.store = store
        self.sessions = store.load_sessions()
        self.send = send
        self.runner = ClaudeStreamRunner()
        self._locks: dict[str, asyncio.Lock] = {}

    def _save(self) -> None:
        self.store.save_sessions(self.sessions)

    def _next_sid(self) -> str:
        nums = []
        for sid in self.sessions:
            try:
                nums.append(int(sid))
            except ValueError:
                pass
        return f"{(max(nums) if nums else 0) + 1:02d}"

    def _default_sid(self) -> str:
        for sid, record in self.sessions.items():
            if record.default:
                return sid
        return ""

    def _set_default(self, sid: str) -> None:
        for record in self.sessions.values():
            record.default = False
        self.sessions[sid].default = True
        self._save()

    async def dispatch(self, text: str) -> str:
        t = " ".join(text.strip().split()) if text.strip().startswith(("/", "#")) else text.strip()
        if not t:
            return ""
        if t in {"/h", "/help", "/helps"}:
            return command_help()
        if t in {"/status", "/st", "/threads", "/th", "#l", "#list"}:
            return self.list_sessions()
        if t in {"/reset", "/r", "#r", "#reset"}:
            for record in self.sessions.values():
                record.default = False
            self._save()
            return "Default session cleared; use /open <sid> to choose one"
        if t in {"/new", "/n", "#n", "#new"}:
            return await self.new_session(".")
        for prefix in ("/new ", "/n ", "#n ", "#new "):
            if t.startswith(prefix):
                return await self.new_session(t[len(prefix):].strip() or ".")
        for prefix in ("/open ", "/o "):
            if t.startswith(prefix):
                sid = t[len(prefix):].strip().split()[0]
                return self.open_session(sid)
        for prefix in ("/s ", "/send "):
            if t.startswith(prefix):
                rest = t[len(prefix):].strip()
                sid, _, msg = rest.partition(" ")
                if not sid or not msg.strip():
                    return "usage: /s <sid> <message>"
                await self.send_to_session(sid, msg.strip())
                return ""
        if t.startswith("#"):
            sid_cmd = t[1:]
            sid, _, msg = sid_cmd.partition(" ")
            if sid and not msg:
                return self.open_session(sid)
            if sid and msg.strip():
                await self.send_to_session(sid, msg.strip())
                return ""
            return "invalid command; use /help"

        sid = self._default_sid()
        if not sid:
            return "No default session set; create one with /new <dir> or choose one with /open <sid>"
        await self.send_to_session(sid, t)
        return ""

    async def new_session(self, work_dir: str) -> str:
        path = Path(os.path.expanduser(work_dir)).resolve()
        if not path.exists() or not path.is_dir():
            return f"directory does not exist: {path}"
        sid = self._next_sid()
        self.sessions[sid] = SessionRecord(
            sid=sid,
            work_dir=str(path),
            claude_session_id=str(uuid.uuid4()),
            default=True,
        )
        self._set_default(sid)
        return f"Session #{sid} created\nDir: {path}\nSet as default; just send messages directly\n(send /reset to clear default)"

    def open_session(self, sid: str) -> str:
        if sid not in self.sessions:
            return f"session #{sid} does not exist; create one with /new <dir>"
        self._set_default(sid)
        return f"#{sid} set as default session; just send messages directly\n(send /reset to clear default)"

    def list_sessions(self) -> str:
        if not self.sessions:
            return "No active sessions. Send /new ~/proj to create one"
        lines = ["Active sessions:"]
        for sid, record in sorted(self.sessions.items()):
            tag = " [default]" if record.default else ""
            lines.append(f"  #{sid}  {record.work_dir}  python-stream{tag}")
        if not self._default_sid():
            lines.append("\nsend /open <sid> to set default session")
        return "\n".join(lines)

    async def send_to_session(self, sid: str, text: str) -> None:
        record = self.sessions.get(sid)
        if not record:
            await self.send(f"error: session #{sid} does not exist; create one with /new <dir>")
            return
        lock = self._locks.setdefault(sid, asyncio.Lock())
        if lock.locked():
            await self.send(f"[{sid}] System\nsession is busy; queued message will run after the current turn")
        async with lock:
            async for chunk in self.runner.run_turn(
                work_dir=record.work_dir,
                session_id=record.claude_session_id,
                prompt=text,
            ):
                await self.send(f"[{sid}] {chunk.sender}\n{chunk.text}")


def command_help() -> str:
    return """ClaudeBridge commands
/help, /h          show this help
/status, /st       list sessions
/new [dir], /n     create a Claude session
/open <sid>, /o    set default session
/s <sid> <text>    send to a specific session
/reset, /r         clear default session

Short # commands still work:
#l, #n ~/project, #<sid>, #<sid> hello, #r""".strip()
