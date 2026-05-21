欢迎关注B站及YouTube频道：深度云创科技，感兴趣的朋友欢迎加入新时代智能体交流社群

客服微信：16773345788

# ClaudeBridge

ClaudeBridge connects Claude Code to a personal WeChat account, so you can start, select, and control Claude sessions from your phone while the real Claude Code process keeps running on your computer.

This repository is the Claude-focused sibling of the WeChat runtime flow used by `codex-bridge`: scan QR once, keep a long-running bridge process alive, route WeChat text to a local coding agent, and push agent output plus permission prompts back to WeChat.

## What It Does

- Personal WeChat QR login through the iLink-compatible `wechatbot` SDK.
- Long-running `serve` loop for receiving WeChat messages and sending replies.
- Multiple Claude Code sessions, each addressable from WeChat.
- Slash-command UX aligned with `codex-bridge`, while keeping the original short `#` commands.
- Claude Code hook integration on macOS for tool output, final answers, notifications, permission requests, elicitation prompts, compaction events, and subagent events.
- Direct Claude stream-json print-mode sessions on Windows, using one stable Claude `--session-id` per WeChat session.
- The same WeChat command surface on Windows and macOS: `/new`, `/open`, `/s`, `/status`, `/reset`, message chunking, typing indicators, durable WeChat context tokens, and stable Claude conversation context.
- Context-token persistence so outbound WeChat replies keep working after bridge restarts.
- Message chunking below WeChat text limits and typing indicators where the transport supports them.

## Runtime Requirements

Supported hosts:

- macOS: full Terminal/FIFO/hook integration, including discovery of locally started Claude sessions.
- Windows: bridge-created Claude sessions via `/new`, using `claude -p --session-id <uuid> --output-format stream-json` per user turn. Sessions are persisted and restored across bridge restarts.

Required:

- Go 1.22 or newer
- Claude Code CLI available as `claude`
- A personal WeChat account that can scan the login QR code

macOS also requires Python 3 for the hook and terminal mux scripts.

## Build

```bash
git clone https://github.com/crazymsn/claude-bridge.git
cd claude-bridge
make cli
```

The binary is written to:

```text
./bin/claude-bridge
```

Windows PowerShell:

```powershell
go build -o .\bin\claude-bridge.exe .\cmd
```

The binary is written to:

```text
.\bin\claude-bridge.exe
```

## First-Time Setup

On macOS, install the Claude hook script:

```bash
./bin/claude-bridge install-hooks
```

Merge the printed JSON into:

```text
~/.claude/settings.json
```

Then scan the WeChat QR code:

```bash
./bin/claude-bridge login
```

On Windows:

```powershell
.\bin\claude-bridge.exe login
```

Start the bridge:

```bash
./bin/claude-bridge start
```

On Windows:

```powershell
.\bin\claude-bridge.exe start
```

For foreground debugging:

```bash
./bin/claude-bridge serve
```

On Windows:

```powershell
.\bin\claude-bridge.exe serve
```

## Local Commands

```bash
claude-bridge             # start bridge in background
claude-bridge start       # same as above
claude-bridge serve       # run bridge in foreground
claude-bridge status      # show bridge status
claude-bridge stop        # stop background bridge
claude-bridge list        # list discoverable Claude sessions
claude-bridge logs        # view recent logs
claude-bridge logs -f     # follow logs
claude-bridge login       # scan QR to log in
claude-bridge install-hooks
claude-bridge clear       # clear sessions, pipes, and logs, keeping login state
```

## WeChat Commands

Recommended slash commands:

| Message | Action |
|---|---|
| `/help` or `/h` | Show command help |
| `/status` or `/st` | List active sessions |
| `/new ~/my-project` or `/n ~/my-project` | Open a new Claude session |
| `/open 01` or `/o 01` | Set session `01` as default |
| `/s 01 hello` | Send a message to session `01` |
| `/allow` or `/al` | Allow the pending Claude permission request |
| `/deny` or `/dn` | Deny the pending Claude permission request |
| `/reset` or `/r` | Clear the default session |
| plain text | Send to the current default session |

Original short commands still work:

| Message | Action |
|---|---|
| `#l` | List active sessions |
| `#n ~/my-project` | Open a new Claude session |
| `#01` | Set session `01` as default |
| `#01 hello` | Send to session `01` |
| `#r` | Clear the default session |

After `/new` or `#n`, the new session is automatically set as the default. If exactly one session is waiting for a permission or option reply, plain text is routed to that pending interaction automatically.

## State Files

Default state lives under:

```text
~/.claude-bridge/
```

Important files:

```text
credentials.json       WeChat login state
context-tokens.json    per-user WeChat context tokens
ambient-user.txt       last WeChat target
bridge.pid             background bridge PID
bridge.log             rotating runtime log
windows-sessions.json  Windows Claude session mappings
```

macOS session pipes and runtime manifests are kept under `/tmp/claude-bridge-*`. Windows bridge-created session mappings are kept in `~/.claude-bridge/windows-sessions.json`, so `claude-bridge list` and bridge restarts can keep using the same Claude conversation IDs.

## Windows Notes

Windows and macOS expose the same WeChat controls. On Windows, create or reopen a bridge-managed Claude session from WeChat:

```text
/new C:\path\to\project
```

The bridge creates a durable Claude session UUID for that WeChat session. Each user message runs:

```text
claude -p --session-id <uuid> --output-format stream-json --include-partial-messages -- "<message>"
```

The stable `--session-id` preserves Claude conversation context while using Claude Code's officially supported non-interactive pipe mode. The stream-json output is forwarded to WeChat as Assistant, Assistant/Tool, or System messages, matching the macOS bridge's event-style feedback as closely as Claude Code's Windows CLI supports.

The macOS `install-hooks` command is still available for Claude Code's Terminal/FIFO hook path. Windows does not need that hook path because its bridge backend uses Claude Code print-mode with stream-json output.

Optional Windows environment variables:

```powershell
$env:CLAUDE_BRIDGE_CLAUDE_BIN = 'C:\Users\Alex\.local\bin\claude.exe'
$env:CLAUDE_BRIDGE_PERMISSION_MODE = 'acceptEdits'
```

## Development

```bash
make cli
go test ./...
```

Windows build check:

```powershell
go test .\...
go build -o .\bin\claude-bridge.exe .\cmd
```

## Credits

This project is based on the original `coderabbit214/claude-bridge` implementation and adapts the command surface and WeChat operating model toward `crazymsn/codex-bridge`.
