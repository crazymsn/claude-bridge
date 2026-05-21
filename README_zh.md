# ClaudeBridge

ClaudeBridge 用个人微信远程控制电脑上的 Claude Code。你可以在手机微信里新建、切换、继续多个 Claude 会话，真实的 Claude Code 进程仍然运行在你的电脑上。

这个仓库是 `codex-bridge` 微信运行逻辑的 Claude 版：扫码登录，保持 `serve` 常驻，把微信文本路由到本地编码 agent，再把 agent 输出、权限请求、选项确认等事件推回微信。

## 能做什么

- 通过 iLink 兼容的 `wechatbot` SDK 扫码登录个人微信。
- 常驻 `serve` 循环，接收微信消息并发送回复。
- 支持多个 Claude Code 会话，每个会话可从微信指定。
- 对齐 `codex-bridge` 的 slash 命令体验，同时保留原来的 `#` 短命令。
- macOS 接入 Claude Code hooks，推送工具输出、最终回复、通知、权限请求、选项输入、压缩上下文和子任务事件。
- Windows 使用 Claude print-mode，每个微信会话绑定一个稳定的 Claude `--session-id`。
- 持久化微信 context token，bridge 重启后仍能继续向最近的微信联系人回复。
- 自动按微信文本长度分片，并在支持时发送 typing 状态。

## 运行环境

支持的宿主设备：

- macOS：完整 Terminal/FIFO/hooks 集成，也支持发现本地已启动的 Claude 会话。
- Windows：支持通过 `/new` 创建的 Claude 会话，每轮消息运行 `claude -p --session-id <uuid>`。

基础要求：

- Go 1.22 或更新版本
- 已安装 Claude Code CLI，并且命令名为 `claude`
- 可扫码登录的个人微信账号

macOS 还需要 Python 3，用于 hook 和终端 mux 脚本。

## 构建

```bash
git clone https://github.com/crazymsn/claude-bridge.git
cd claude-bridge
make cli
```

生成的二进制文件在：

```text
./bin/claude-bridge
```

Windows PowerShell：

```powershell
go build -o .\bin\claude-bridge.exe .\cmd
```

生成的二进制文件在：

```text
.\bin\claude-bridge.exe
```

## 首次初始化

macOS 先安装 Claude hook 脚本：

```bash
./bin/claude-bridge install-hooks
```

把命令打印出来的 JSON 合并到：

```text
~/.claude/settings.json
```

然后扫码登录微信：

```bash
./bin/claude-bridge login
```

Windows：

```powershell
.\bin\claude-bridge.exe login
```

后台启动 bridge：

```bash
./bin/claude-bridge start
```

Windows：

```powershell
.\bin\claude-bridge.exe start
```

调试时可以前台运行：

```bash
./bin/claude-bridge serve
```

Windows：

```powershell
.\bin\claude-bridge.exe serve
```

## 本地命令

```bash
claude-bridge             # 后台启动 bridge
claude-bridge start       # 同上
claude-bridge serve       # 前台运行 bridge
claude-bridge status      # 查看 bridge 状态
claude-bridge stop        # 停止后台 bridge
claude-bridge list        # 查看当前可发现的 Claude 会话
claude-bridge logs        # 查看最近日志
claude-bridge logs -f     # 持续跟随日志
claude-bridge login       # 扫码登录微信
claude-bridge install-hooks
claude-bridge clear       # 清除会话、管道和日志，保留登录状态
```

## 微信命令

推荐使用 slash 命令：

| 发送内容 | 效果 |
|---|---|
| `/help` 或 `/h` | 查看命令帮助 |
| `/status` 或 `/st` | 列出活跃会话 |
| `/new ~/my-project` 或 `/n ~/my-project` | 打开一个新的 Claude 会话 |
| `/open 01` 或 `/o 01` | 把 `01` 设为默认会话 |
| `/s 01 hello` | 向 `01` 会话发送消息 |
| `/allow` 或 `/al` | 同意当前 Claude 权限请求 |
| `/deny` 或 `/dn` | 拒绝当前 Claude 权限请求 |
| `/reset` 或 `/r` | 清除默认会话 |
| 普通文本 | 发给当前默认会话 |

原来的短命令仍然可用：

| 发送内容 | 效果 |
|---|---|
| `#l` | 列出活跃会话 |
| `#n ~/my-project` | 打开一个新的 Claude 会话 |
| `#01` | 把 `01` 设为默认会话 |
| `#01 hello` | 向 `01` 会话发送消息 |
| `#r` | 清除默认会话 |

`/new` 或 `#n` 创建成功后，会自动把新会话设为默认会话。如果当前只有一个会话正在等待权限或选项回复，普通文本也会自动提交给它。

## 状态文件

默认状态目录：

```text
~/.claude-bridge/
```

重要文件：

```text
credentials.json       微信登录状态
context-tokens.json    每个微信联系人的 context token
ambient-user.txt       最近一次微信目标
bridge.pid             后台 bridge PID
bridge.log             运行日志，会自动轮转
```

macOS 会话管道和运行时 manifest 放在 `/tmp/claude-bridge-*`。Windows 通过 bridge 创建的会话保存在运行中的 bridge 进程里，进程退出后不可发现。

## Windows 说明

Windows 支持的重点是通过微信创建会话：

```text
/new C:\path\to\project
```

bridge 会为该微信会话创建一个 Claude session UUID。每条用户消息会运行：

```text
claude -p --session-id <uuid> -- "<message>"
```

稳定的 `--session-id` 用来保持 Claude 上下文，同时使用 Claude Code 官方支持的非交互 pipe 模式。

当前 Windows 限制：

- `install-hooks` 主要用于 macOS hook 集成。
- `claude-bridge list` 只显示 macOS 文件系统可发现的会话；Windows 进程内会话在 `serve` 运行期间可通过 `/status` 查看。
- 暂不支持挂接已经在 Windows 终端里手动打开的 Claude 会话。
- Windows 不像 macOS hooks 那样流式推送工具生命周期事件；它会在每轮 `claude -p` 完成后返回该轮输出。

Windows 可选环境变量：

```powershell
$env:CLAUDE_BRIDGE_CLAUDE_BIN = 'C:\Users\Alex\.local\bin\claude.exe'
$env:CLAUDE_BRIDGE_PERMISSION_MODE = 'acceptEdits'
```

## 开发

```bash
make cli
go test ./...
```

Windows 构建检查：

```powershell
go test .\...
go build -o .\bin\claude-bridge.exe .\cmd
```

## 致谢

本项目基于 `coderabbit214/claude-bridge` 原始实现，并将命令面和微信运行方式向 `crazymsn/codex-bridge` 对齐。
