from __future__ import annotations

import asyncio
import json
import os
import sys
from collections.abc import AsyncIterator
from dataclasses import dataclass
from pathlib import Path


@dataclass
class ClaudeChunk:
    sender: str
    text: str
    final: bool = False


def _iter_text(value) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        text = value.strip()
        return [text] if text else []
    if isinstance(value, (int, float, bool)):
        return [str(value)]
    if isinstance(value, list):
        parts: list[str] = []
        for item in value:
            parts.extend(_iter_text(item))
        return parts
    if not isinstance(value, dict):
        return []

    block_type = value.get("type")
    if block_type in {"text", "output_text"}:
        return _iter_text(value.get("text"))
    if block_type in {"tool_result", "tool_output"}:
        parts: list[str] = []
        for key in ("output", "stdout", "stderr", "error", "result", "content"):
            parts.extend(_iter_text(value.get(key)))
        return parts

    parts: list[str] = []
    for key in ("stdout", "stderr", "output", "error", "result", "message", "content", "text"):
        parts.extend(_iter_text(value.get(key)))
    return parts


def _unique_join(parts: list[str]) -> str:
    seen: set[str] = set()
    out: list[str] = []
    for part in parts:
        p = part.strip()
        if p and p not in seen:
            seen.add(p)
            out.append(p)
    return "\n\n".join(out)


def _event_to_chunk(event: dict) -> ClaudeChunk | None:
    event_type = str(event.get("type") or "")
    subtype = str(event.get("subtype") or "")

    if event_type in {"assistant", "message"}:
        message = event.get("message") if isinstance(event.get("message"), dict) else event
        text = _unique_join(_iter_text(message.get("content")))
        if text:
            return ClaudeChunk("Assistant", text)

    if event_type in {"result", "final"}:
        text = _unique_join(_iter_text(event.get("result") or event.get("content") or event.get("message")))
        if text:
            return ClaudeChunk("Assistant", text, final=True)

    if "tool" in event_type or "tool" in subtype:
        tool = event.get("name") or event.get("tool_name") or event.get("toolName") or "tool"
        parts = _iter_text(event.get("result") or event.get("tool_response") or event.get("content"))
        body = _unique_join(parts)
        if body:
            return ClaudeChunk("Assistant/Tool", f"{tool}\n{body}")
        return ClaudeChunk("Assistant/Tool", str(tool))

    if event_type in {"error", "system"} or subtype in {"error", "error_max_turns"}:
        text = _unique_join(_iter_text(event.get("error") or event.get("message") or event))
        if text:
            return ClaudeChunk("System", text)

    return None


class ClaudeStreamRunner:
    def __init__(self, claude_bin: str | None = None) -> None:
        self.claude_bin = claude_bin or os.environ.get("CLAUDE_BRIDGE_CLAUDE_BIN") or "claude"

    async def run_turn(
        self,
        *,
        work_dir: str,
        session_id: str,
        prompt: str,
    ) -> AsyncIterator[ClaudeChunk]:
        args = [
            self.claude_bin,
            "-p",
            "--session-id",
            session_id,
            "--output-format",
            "stream-json",
            "--include-partial-messages",
        ]
        permission_mode = os.environ.get("CLAUDE_BRIDGE_PERMISSION_MODE", "").strip()
        if permission_mode:
            args.extend(["--permission-mode", permission_mode])
        args.extend(["--", prompt])

        proc = await asyncio.create_subprocess_exec(
            *args,
            cwd=str(Path(work_dir)),
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )

        assert proc.stdout is not None
        assert proc.stderr is not None

        async def read_stderr() -> str:
            data = await proc.stderr.read()
            return data.decode(errors="replace").strip()

        stderr_task = asyncio.create_task(read_stderr())
        last_text = ""
        async for raw in proc.stdout:
            line = raw.decode(errors="replace").strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                yield ClaudeChunk("Assistant", line)
                continue
            chunk = _event_to_chunk(event)
            if not chunk or not chunk.text:
                continue
            if chunk.sender == "Assistant" and chunk.text.startswith(last_text):
                delta = chunk.text[len(last_text):].strip()
                last_text = chunk.text
                if delta:
                    yield ClaudeChunk(chunk.sender, delta, chunk.final)
                continue
            if chunk.sender == "Assistant":
                last_text = chunk.text
            yield chunk

        code = await proc.wait()
        stderr = await stderr_task
        if code != 0:
            detail = stderr or f"claude exited with code {code}"
            yield ClaudeChunk("System", detail, final=True)
