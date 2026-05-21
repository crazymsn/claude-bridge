from __future__ import annotations

import argparse
import asyncio
import os
import signal
import subprocess
import sys
import time
from contextlib import suppress
from pathlib import Path

from .sessions import SessionManager
from .state import StateStore, default_state_dir
from .wechat_runtime import WeChatRuntime


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(prog="claude-bridge")
    parser.add_argument("--state-dir", default=None)
    sub = parser.add_subparsers(dest="command")
    sub.add_parser("login")
    sub.add_parser("serve")
    sub.add_parser("start")
    sub.add_parser("stop")
    sub.add_parser("status")
    sub.add_parser("logs")
    sub.add_parser("list")
    sub.add_parser("clear")
    args = parser.parse_args(argv)
    command = args.command or "start"
    store = StateStore(Path(args.state_dir).expanduser().resolve() if args.state_dir else default_state_dir())

    if command == "login":
        asyncio.run(cmd_login(store))
    elif command == "serve":
        asyncio.run(cmd_serve(store))
    elif command == "start":
        cmd_start(store)
    elif command == "stop":
        cmd_stop(store)
    elif command == "status":
        cmd_status(store)
    elif command == "logs":
        cmd_logs(store)
    elif command == "list":
        cmd_list(store)
    elif command == "clear":
        cmd_clear(store)
    else:
        parser.print_help()


async def cmd_login(store: StateStore) -> None:
    runtime = WeChatRuntime(store)
    await runtime.login(force=True)


async def cmd_serve(store: StateStore) -> None:
    store.pid_path.write_text(str(os.getpid()) + "\n", "utf-8")
    runtime = WeChatRuntime(store)
    manager_holder: dict[str, SessionManager] = {}

    async def send(text: str) -> None:
        await runtime.send_to_target(text)

    manager_holder["manager"] = SessionManager(store, send)

    @runtime.bot.on_message
    async def handle(msg):
        if not msg.text:
            return
        runtime.set_target(msg.user_id)
        await runtime.typing(msg.user_id, True)
        try:
            manager = manager_holder["manager"]
            reply = await manager.dispatch(msg.text)
            if reply:
                await runtime.reply(msg, reply)
        except Exception as exc:
            await runtime.reply(msg, "error: " + str(exc))
        finally:
            await runtime.typing(msg.user_id, False)

    try:
        await runtime.start()
    finally:
        with suppress(FileNotFoundError):
            store.pid_path.unlink()


def cmd_start(store: StateStore) -> None:
    pid = _read_pid(store.pid_path)
    if pid and _pid_alive(pid):
        print(f"bridge is already running in background, pid={pid}")
        return
    store.root.mkdir(parents=True, exist_ok=True)
    log = open(store.log_path, "ab", buffering=0)
    cmd = [sys.executable, "-m", "claude_bridge", "--state-dir", str(store.root), "serve"]
    kwargs = dict(stdin=subprocess.DEVNULL, stdout=log, stderr=log, cwd=str(Path.cwd()))
    if os.name == "nt":
        kwargs["creationflags"] = subprocess.CREATE_NO_WINDOW | subprocess.DETACHED_PROCESS
    else:
        kwargs["start_new_session"] = True
    proc = subprocess.Popen(cmd, **kwargs)
    store.pid_path.write_text(str(proc.pid) + "\n", "utf-8")
    print(f"bridge started, pid={proc.pid}")
    print(f"log file: {store.log_path}")


def cmd_stop(store: StateStore) -> None:
    pid = _read_pid(store.pid_path)
    if not pid:
        print("bridge is not running")
        return
    try:
        if os.name == "nt":
            subprocess.run(["taskkill", "/PID", str(pid), "/T", "/F"], check=False, stdout=subprocess.DEVNULL)
        else:
            os.kill(pid, signal.SIGTERM)
        print(f"bridge stopped, pid={pid}")
    finally:
        with suppress(FileNotFoundError):
            store.pid_path.unlink()


def cmd_status(store: StateStore) -> None:
    pid = _read_pid(store.pid_path)
    if pid and _pid_alive(pid):
        print(f"bridge is running, pid={pid}")
    else:
        print("bridge is not running")


def cmd_list(store: StateStore) -> None:
    manager = SessionManager(store, _noop_send)
    print(manager.list_sessions())


def cmd_logs(store: StateStore) -> None:
    try:
        print(store.log_path.read_text("utf-8", errors="replace")[-20000:])
    except FileNotFoundError:
        print(f"log file not found: {store.log_path}")


def cmd_clear(store: StateStore) -> None:
    cmd_stop(store)
    for path in (store.sessions_path, store.log_path):
        with suppress(FileNotFoundError):
            path.unlink()
            print(f"removed: {path}")


async def _noop_send(text: str) -> None:
    return None


def _read_pid(path: Path) -> int | None:
    try:
        return int(path.read_text("utf-8").strip())
    except (FileNotFoundError, ValueError):
        return None


def _pid_alive(pid: int) -> bool:
    if os.name == "nt":
        result = subprocess.run(["tasklist", "/FI", f"PID eq {pid}", "/NH"], capture_output=True, text=True)
        return str(pid) in result.stdout and "No tasks are running" not in result.stdout
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False
