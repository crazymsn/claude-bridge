from __future__ import annotations

import asyncio
import sys
from collections.abc import Awaitable, Callable
from typing import TYPE_CHECKING

from .state import StateStore

MAX_CHUNK_CHARS = 3900

if TYPE_CHECKING:
    from wechatbot import IncomingMessage, WeChatBot


class WeChatRuntime:
    def __init__(self, store: StateStore) -> None:
        from wechatbot import WeChatBot

        self.store = store
        self.bot = WeChatBot(
            cred_path=str(store.credentials_path),
            on_qr_url=self._on_qr_url,
            on_scanned=lambda: print("QR scanned, waiting for confirmation...", file=sys.stderr),
            on_error=lambda err: print(f"[wechat] {err}", file=sys.stderr),
        )
        self.target_user = store.load_target_user()

    def _on_qr_url(self, url: str) -> None:
        print("Scan this QR URL with WeChat:", file=sys.stderr)
        print(url, file=sys.stderr)

    async def login(self, *, force: bool = False) -> None:
        creds = await self.bot.login(force=force)
        print(f"WeChat connected! account_id={creds.account_id}")
        print(f"Credentials saved to: {self.store.credentials_path}")

    def on_message(self, handler: Callable[["IncomingMessage"], Awaitable[None]]) -> None:
        self.bot.on_message(handler)

    async def start(self) -> None:
        await self.bot.login(force=False)
        await self.bot.start()

    async def reply(self, msg: "IncomingMessage", text: str) -> None:
        self.set_target(msg.user_id)
        for chunk in _split_message(text):
            await self.bot.reply(msg, chunk)

    async def send_to_target(self, text: str) -> None:
        if not self.target_user:
            print("[wechat] no target user yet; dropping outbound message", file=sys.stderr)
            return
        for chunk in _split_message(text):
            await self.bot.send(self.target_user, chunk)

    def set_target(self, user_id: str) -> None:
        self.target_user = user_id
        self.store.save_target_user(user_id)

    async def typing(self, user_id: str, enabled: bool) -> None:
        try:
            if enabled:
                await self.bot.send_typing(user_id)
            else:
                await self.bot.stop_typing(user_id)
        except Exception:
            pass


def _split_message(text: str) -> list[str]:
    if not text:
        return [""]
    chunks: list[str] = []
    remaining = text
    while len(remaining) > MAX_CHUNK_CHARS:
        cut = remaining.rfind("\n", 0, MAX_CHUNK_CHARS)
        if cut <= 0:
            cut = MAX_CHUNK_CHARS
        chunks.append(remaining[:cut])
        remaining = remaining[cut:].lstrip("\n")
    chunks.append(remaining)
    return chunks
