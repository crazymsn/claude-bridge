# ClaudeBridge

ClaudeBridge connects Claude Code to a personal WeChat account. It is now a Python-only project: the same Python runtime is used on Windows and macOS, with no Go build step and no platform-specific feature downgrade.

## What It Does

- Personal WeChat QR login through the `wechatbot` SDK.
- Long-running `serve` loop for receiving WeChat messages and sending replies.
- Multiple Claude Code sessions, each addressable from WeChat.
- Stable Claude conversation context through one `--session-id` per bridge session.
- Streamed Claude output via `claude -p --output-format stream-json`.
- The same WeChat command surface on Windows and macOS: `/new`, `/open`, `/s`, `/status`, `/reset`, typing indicators, message replies, and durable session state.

## Requirements

- Python 3.11 or newer.
- Claude Code CLI available as `claude`.
- A personal WeChat account that can scan the login QR code.

## Install

```bash
git clone https://github.com/crazymsn/claude-bridge.git
cd claude-bridge
python -m pip install -e .
```

After installation, the command is:

```bash
claude-bridge
```

You can also run it without installing:

```bash
python -m claude_bridge
```

## First-Time Setup

Scan the WeChat QR code:

```bash
claude-bridge login
```

Start the bridge in the background:

```bash
claude-bridge start
```

For foreground debugging:

```bash
claude-bridge serve
```

Windows and macOS use the same commands. If your Claude binary is not named `claude`, set:

```powershell
$env:CLAUDE_BRIDGE_CLAUDE_BIN = 'C:\path\to\claude.exe'
```

On macOS/Linux:

```bash
export CLAUDE_BRIDGE_CLAUDE_BIN=/path/to/claude
```

## Local Commands

```bash
claude-bridge             # start bridge in background
claude-bridge start       # same as above
claude-bridge serve       # run bridge in foreground
claude-bridge status      # show bridge process status
claude-bridge stop        # stop the background bridge
claude-bridge list        # list persisted Claude sessions
claude-bridge logs        # view recent logs
claude-bridge login       # scan QR to log in
claude-bridge clear       # clear sessions and logs, keeping WeChat credentials
```

## WeChat Commands

| Message | Action |
|---|---|
| `/help` or `/h` | Show command help |
| `/status` or `/st` | List sessions |
| `/new ~/my-project` or `/n ~/my-project` | Create a Claude session |
| `/open 01` or `/o 01` | Set session `01` as default |
| `/s 01 hello` | Send a message to session `01` |
| `/reset` or `/r` | Clear the default session |
| plain text | Send to the current default session |

Original short commands still work:

| Message | Action |
|---|---|
| `#l` | List sessions |
| `#n ~/my-project` | Create a Claude session |
| `#01` | Set session `01` as default |
| `#01 hello` | Send to session `01` |
| `#r` | Clear the default session |

## State Files

Default state lives under:

```text
~/.claude-bridge/
```

Important files:

```text
credentials.json       WeChat login state
ambient-user.txt       last WeChat target
bridge.pid             background bridge PID
bridge.log             runtime log
python-sessions.json   Claude session mappings
```

`python-sessions.json` is used on both Windows and macOS. The stable Claude `--session-id` values inside it let the bridge keep conversation context after restarts.

## Development

```bash
python -m compileall claude_bridge
python -m claude_bridge --help
```

## Credits

This project is based on the original `coderabbit214/claude-bridge` idea and adapts the command surface and WeChat operating model toward `crazymsn/codex-bridge`.
