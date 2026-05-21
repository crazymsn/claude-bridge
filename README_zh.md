# ClaudeBridge

ClaudeBridge 用个人微信远程控制电脑上的 Claude Code。项目现在已经统一为 Python 实现：Windows 和 macOS 使用同一套 Python 运行时，不再需要 Go，也没有平台功能缩水。

## 能做什么

- 通过 `wechatbot` SDK 扫码登录个人微信。
- 常驻 `serve` 循环，接收微信消息并发送回复。
- 支持多个 Claude Code 会话，每个会话都可以从微信指定。
- 每个 bridge 会话绑定一个稳定的 Claude `--session-id`，用于保持上下文。
- 通过 `claude -p --output-format stream-json` 持续推送 Claude 输出。
- Windows 和 macOS 使用同一套微信命令：`/new`、`/open`、`/s`、`/status`、`/reset`、typing 状态、消息回复和持久化会话状态。

## 运行要求

- Python 3.11 或更新版本。
- 已安装 Claude Code CLI，并且命令名为 `claude`。
- 可扫码登录的个人微信账号。

## 安装

```bash
git clone https://github.com/crazymsn/claude-bridge.git
cd claude-bridge
python -m pip install -e .
```

安装后命令为：

```bash
claude-bridge
```

也可以不安装，直接运行：

```bash
python -m claude_bridge
```

## 首次设置

扫码登录微信：

```bash
claude-bridge login
```

后台启动 bridge：

```bash
claude-bridge start
```

调试时前台运行：

```bash
claude-bridge serve
```

Windows 和 macOS 使用同样命令。如果你的 Claude 命令不是 `claude`，可以设置：

```powershell
$env:CLAUDE_BRIDGE_CLAUDE_BIN = 'C:\path\to\claude.exe'
```

macOS/Linux：

```bash
export CLAUDE_BRIDGE_CLAUDE_BIN=/path/to/claude
```

## 本地命令

```bash
claude-bridge             # 后台启动 bridge
claude-bridge start       # 同上
claude-bridge serve       # 前台运行 bridge
claude-bridge status      # 查看 bridge 进程状态
claude-bridge stop        # 停止后台 bridge
claude-bridge list        # 查看已持久化的 Claude 会话
claude-bridge logs        # 查看最近日志
claude-bridge login       # 扫码登录微信
claude-bridge clear       # 清除会话和日志，保留微信登录状态
```

## 微信命令

| 发送内容 | 效果 |
|---|---|
| `/help` 或 `/h` | 查看命令帮助 |
| `/status` 或 `/st` | 列出会话 |
| `/new ~/my-project` 或 `/n ~/my-project` | 创建 Claude 会话 |
| `/open 01` 或 `/o 01` | 把 `01` 设为默认会话 |
| `/s 01 hello` | 向 `01` 会话发送消息 |
| `/reset` 或 `/r` | 清除默认会话 |
| 普通文本 | 发给当前默认会话 |

原来的短命令仍然可用：

| 发送内容 | 效果 |
|---|---|
| `#l` | 列出会话 |
| `#n ~/my-project` | 创建 Claude 会话 |
| `#01` | 把 `01` 设为默认会话 |
| `#01 hello` | 向 `01` 会话发送消息 |
| `#r` | 清除默认会话 |

## 状态文件

默认状态目录：

```text
~/.claude-bridge/
```

重要文件：

```text
credentials.json       微信登录状态
ambient-user.txt       最近一次微信目标
bridge.pid             后台 bridge PID
bridge.log             运行日志
python-sessions.json   Claude 会话映射
```

Windows 和 macOS 都使用 `python-sessions.json`。其中保存的稳定 Claude `--session-id` 可以让 bridge 重启后继续同一个 Claude 上下文。

## 开发

```bash
python -m compileall claude_bridge
python -m claude_bridge --help
```

## 致谢

本项目基于 `coderabbit214/claude-bridge` 的原始思路，并将命令面和微信运行方式向 `crazymsn/codex-bridge` 对齐。
