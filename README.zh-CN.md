# notion-local-ops-mcp

[English](./README.md)

把 Notion 里的 **MCP Agent** 变成一个可操作本地文件、shell、git 和长任务轮询的 coding agent。

![在本地仓库中工作的 MCP Agent](./assets/notion/notion-handoff-chat.png)

## 当前状态

- 默认运行时已经切到仓库根目录的 Go 服务
- `./scripts/dev-tunnel.sh` 现在负责 `go run ./main.go` + `ngrok`
- 仓库现在只保留 Go MCP 运行时，不再依赖 Python 服务端
- supervisor / launchd / cloudflared 已不再属于支持的运行路径

## 准备工作

Windows 用户请在 **Git Bash** 或 **WSL** 里执行本文档中的 shell 命令。

| 工具 | 用途 | 检查 |
| --- | --- | --- |
| Go | 运行默认 MCP 服务 | `go version` |
| ngrok | 把本地 MCP 服务通过 HTTPS 暴露出去 | `ngrok version` |
| Git | 克隆和管理仓库 | `git --version` |

## 快速开始

### 1. 克隆并配置

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp
cp .env.example .env
```

至少填写：

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

### 2. 启动本地服务和 ngrok tunnel

```bash
./scripts/dev-tunnel.sh
```

脚本会：

- 在 `http://127.0.0.1:8766/mcp` 启动 Go MCP 服务
- 执行 `ngrok http http://127.0.0.1:8766`
- tunnel 就绪后打印 `Public MCP URL: https://.../mcp`

连接 Notion 时，这个终端需要保持运行。

### 3. 在 Notion 里配置 MCP Agent

- URL：`https://<你的-ngrok-域名>/mcp`
- Auth type：`Bearer`
- Token：`NOTION_LOCAL_OPS_AUTH_TOKEN`

## 只运行 `main.go`

如果你只想起本地服务，公网 tunnel 由自己管理：

```bash
go run ./main.go
```

现在 `go run ./main.go` 会从当前工作目录开始向上查找最近的 `.env` 并自动加载；`.env` 中已定义的值优先级高于同名 shell 环境变量。

PowerShell 示例：

```powershell
$env:NOTION_LOCAL_OPS_HOST='127.0.0.1'
$env:NOTION_LOCAL_OPS_PORT='8766'
$env:NOTION_LOCAL_OPS_WORKSPACE_ROOT='C:\absolute\path\to\workspace'
$env:NOTION_LOCAL_OPS_STATE_DIR='C:\absolute\path\to\state-dir'
$env:NOTION_LOCAL_OPS_AUTH_TOKEN='replace-me'
go run .\main.go
```

## `dev-tunnel.sh` 约定

支持的动作只有：

- `./scripts/dev-tunnel.sh`
- `./scripts/dev-tunnel.sh start`
- `./scripts/dev-tunnel.sh status`

`status` 会输出：

- 本地 MCP 端点是否可达
- 当前 ngrok 公网 URL，如果 ngrok 本地 API 可访问

不再支持 `reload`。

## 环境变量

核心运行时：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_HOST` | `127.0.0.1` | 绑定地址 |
| `NOTION_LOCAL_OPS_PORT` | `8766` | 绑定端口 |
| `NOTION_LOCAL_OPS_WORKSPACE_ROOT` | `$HOME` | 相对路径锚点 |
| `NOTION_LOCAL_OPS_STATE_DIR` | `~/.notion-local-ops-mcp` | 任务元数据目录 |
| `NOTION_LOCAL_OPS_AUTH_TOKEN` | empty | Bearer token |

ngrok：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_NGROK_COMMAND` | `ngrok` | ngrok CLI 路径 |
| `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN` | empty | 项目内的 ngrok token，会桥接到 `NGROK_AUTHTOKEN` |
| `NOTION_LOCAL_OPS_NGROK_DOMAIN` | empty | 可选，保留域名 |
| `NOTION_LOCAL_OPS_NGROK_REGION` | empty | 可选，ngrok region |
| `NOTION_LOCAL_OPS_NGROK_API_URL` | `http://127.0.0.1:4040/api/tunnels` | `status` 用的本地 ngrok API |

如果你机器上本来就有全局环境变量 `NGROK_AUTHTOKEN`，只有在 `.env` 没有定义 `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN` 时，`./scripts/dev-tunnel.sh` 才会复用它；如果你希望把 token 放在当前仓库的 `.env` 里，就设置 `NOTION_LOCAL_OPS_NGROK_AUTHTOKEN`，脚本会在启动 ngrok 前把它导出成 `NGROK_AUTHTOKEN`。

任务 / shell：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `NOTION_LOCAL_OPS_COMMAND_TIMEOUT` | `120` | 前台 shell 超时秒数 |
| `NOTION_LOCAL_OPS_DEBUG_MCP_LOGGING` | `0` | MCP 请求调试日志 |
| `NOTION_LOCAL_OPS_EXTRA_WRITE_DIRS` | empty | 额外允许写入的目录，容器里常用于 `/tmp` |

如果你要用 ChatGPT Web OAuth，`.env.example` 里保留的 OAuth 变量依然有效，只需要把 `NOTION_LOCAL_OPS_PUBLIC_BASE_URL` 指到你的 ngrok HTTPS 地址。

## 默认 Go 工具集

现在默认 Go 运行时已经覆盖日常替代 Python 版所需的 MCP 工具面：

- `server_info`
- `list_skills`
- `set_default_cwd`
- `get_default_cwd`
- `list_files`
- `read_text`
- `write_file`
- `search`
- `apply_patch`
- `run_command`
- `run_command_stream`
- `open_shell_session`
- `get_shell_session`
- `send_shell_input`
- `read_shell_output`
- `close_shell_session`
- `tcp_connect`
- `tcp_send`
- `tcp_read`
- `tcp_close`
- `wait_task`
- `get_task`
- `get_task_logs`
- `cancel_task`
- `purge_tasks`
- `git_status`
- `git_diff`
- `git_commit`
- `git_log`
- `git_show`
- `git_blame`

仓库已经不再依赖 Python MCP 实现。

## 持久 Shell Session

Phase 1 新增了 Linux 优先的 PTY 持久 shell session，用来覆盖需要保持终端状态的交互式流程：

1. `open_shell_session` 在解析后的工作目录中启动持久 shell。
2. `send_shell_input` 向该 PTY 写入命令。
3. `read_shell_output` 通过 `offset` / `next_offset` 增量读取持久化输出。
4. `get_shell_session` 返回会话元数据和当前是否仍然 active。
5. `close_shell_session` 终止 PTY 并把会话标记为 closed。

适合的场景包括：多次 `cd`、导出环境变量、REPL、`nc`、`socat`、Python、以及利用脚本等需要保留上下文的工作流。

当前 Phase 1 限制：

- 只在 Linux / 容器运行时完整支持
- 非 Linux 平台会明确返回 unsupported 错误
- session 与 background task 分开存储，状态落在 `STATE_DIR/sessions`

## 原生 TCP 工具（Phase 2，仅 TCP）

Phase 2 增加了面向 CTF 交互服务的原生持久 TCP 工作流：

1. `tcp_connect` 建立出站 TCP 连接，并返回持久 `connection_id`。
2. `tcp_send` 向该连接写入数据。
3. `tcp_read` 按超时、最大字节数、可选分隔符做增量读取。
4. `tcp_close` 关闭连接并标记为 inactive。

载荷规则：

- `tcp_send` 每次调用只能使用一种载荷模式：
  - `text`：普通文本
  - `content_base64`：原始字节（base64 编码传输）
- `append_newline` 用于 text 模式下便捷追加换行。
- `tcp_read` 默认输出 text；需要按字节安全传输时可设置 `output_mode: "base64"`。

分隔符规则：

- `read_until`：文本分隔符匹配
- `read_until_base64`：字节分隔符（base64 编码）
- 一次 `tcp_read` 只应使用一种分隔符字段

持久化与重启行为：

- 连接元数据和 I/O 日志会持久化到 `STATE_DIR/connections`。
- 活跃 socket 仅存在于当前进程内，服务重启后不会自动恢复。
- 重启后元数据仍可查看，但旧连接不再 active。

实用 MCP 流程示例：

```json
{
  "name": "tcp_connect",
  "host": "127.0.0.1",
  "port": 31337,
  "timeout_seconds": 2
}
```

然后按顺序调用：

1. `tcp_read` 读取初始 banner / prompt
2. `tcp_send` 发送 `text`（或按需发送 `content_base64`）
3. `tcp_read` 再次读取，必要时配合 `read_until` / `read_until_base64`
4. 结束后调用 `tcp_close`

Phase 2 范围说明：

- 本节仅覆盖 TCP。
- UDP、TLS 专用工具、以及 server/listener 模式都不在 Phase 2 范围内。

## 长任务命令流

短命令、希望在一次 MCP 请求里直接拿到结果时，用 `run_command`。

耗时命令、需要轮询进度或日志时，用 `run_command_stream`。它会立刻返回 `task_id`，之后优先用 `wait_task`，不要高频忙轮询 `get_task`。

### 命令输入建议

如果命令需要传大段文本，或者输入内容对引号、换行、转义比较敏感，优先通过请求字段把内容送进标准输入，不要依赖 heredoc 或复杂 shell 转义。

- `run_command` 支持 `stdin`
- `run_command` 也支持 `stdin_content`，作为更明确的别名字段

两个字段都会走同一条标准输入通路，二选一即可。

推荐轮询流程：

1. 先调用 `run_command_stream`，保存返回的 `task_id`。
2. 调用 `wait_task`，并把上一次响应里的 `event_seq` 作为 `last_event_seq` 传回去。
3. 如果任务仍在运行，就按 `recommended_poll_strategy` 和 `next_poll_after_seconds` 继续轮询。
4. 需要完整任务快照时，再调用 `get_task`。
5. 只有在需要看 stdout/stderr 细节时，才调用 `get_task_logs`。

### 网页端长任务恢复

在网页端实时对话里，长任务可能跨越当前轮次，甚至在这次对话结束后还没完成。面向用户的跟进优先使用 `await_task`，这样服务端会代替客户端持续等待，不需要在网页端反复做短周期 `wait_task` 轮询。

如果当前对话已经结束，推荐按 `list_recent_tasks` -> `get_task` -> `await_task` -> `get_task_logs` 的顺序恢复。发给用户的后续提示里应包含 `resume_token` 或 `task_id`，方便下一轮继续接上同一个任务。

`get_task_logs` 请求示例：

```json
{
  "name": "get_task_logs",
  "task_id": "task-123",
  "stream": "stdout",
  "offset": 0,
  "limit": 4096
}
```

`get_task_logs` 会按 `offset` / `next_offset` / `truncated` 返回增量日志切片。某个流暂时没有日志文件时，会按“空日志但成功”处理，而不是直接报错。

如果要取消任务，调用 `cancel_task` 并传入同一个 `task_id`。只有在执行确实被中断时，任务才会进入 `cancelled`。

### `write_file` 内容模式

`write_file` 需要提供 `path`，并且以下两个内容字段必须且只能提供一个：

- `content`：普通文本写入
- `content_base64`：服务端先做 base64 解码，再按字节原样写入

两种模式都支持 `dry_run`。非法 base64，或者同时传两个内容字段，都会返回校验错误。

Python 脚本可以通过 stdout 输出结构化进度：

```text
MCP_PROGRESS {"percent": 25, "message": "正在扫描工作区"}
MCP_PROGRESS {"percent": 80, "message": "正在聚合结果"}
```

Go runner 会解析这些行，并通过 `progress_percent`、`progress_message` 和 `event_seq` 对外暴露。

## 验证

Go 侧测试：

```bash
$env:GOCACHE=(Join-Path (Get-Location) '.gocache'); go test ./go/internal/tools ./go/internal/taskstore ./go/internal/mcp ./go/internal/app -count=1
```

手动烟测：

```bash
go run ./main.go
```

然后访问 `http://127.0.0.1:8766/mcp`，或者执行 `./scripts/dev-tunnel.sh status`。

## 容器部署

这个仓库现在可以直接打包成 Docker 容器运行，不再依赖 ngrok。镜像里只会复制 MCP 服务二进制，并把容器内默认工作区设置为 `/tmp`，然后以如下配置启动：

- `NOTION_LOCAL_OPS_WORKSPACE_ROOT=/tmp`
- `NOTION_LOCAL_OPS_STATE_DIR=/tmp/notion-local-ops-mcp`
- `HOME=/tmp`

附带的 [`docker-compose.yml`](./docker-compose.yml) 还会设置：

- `read_only: true`
- `tmpfs: /tmp`
- `NOTION_LOCAL_OPS_EXTRA_WRITE_DIRS=/tmp`
- `cap_drop: [ALL]`
- `no-new-privileges:true`

这意味着：

- 命令只会在容器内执行
- 不挂载宿主机目录
- 默认工作目录就是容器内本地的 `/tmp`
- `/tmp` 保持可执行，便于 PTY shell 和临时工具链使用
- 任务状态不持久化
- 重启或重建容器后，运行态会全部还原

Phase 1 运行时镜像还补上了最小可用的 CTF 工具链基线：

- `file`
- `strings` / `objdump` / `readelf`（来自 `binutils`）
- `gdb`
- Python 打包支持
- `pwntools`
- `ROPGadget`

构建并启动：

```bash
docker compose up --build -d
```

上线前请设置真实 Bearer Token：

```yaml
environment:
  NOTION_LOCAL_OPS_AUTH_TOKEN: your-strong-token
```

服务端现在也会强制工作区约束：解析出的路径和 session cwd 不能逃出当前配置的 workspace root，即使是绝对路径也会被拒绝。

## 故障排查

### Notion 提示无法连接

- 确认 `./scripts/dev-tunnel.sh` 还在运行
- 确认填进 Notion 的 URL 以 `/mcp` 结尾
- 确认 Token 和 `NOTION_LOCAL_OPS_AUTH_TOKEN` 完全一致

### 本地 MCP 通，公网 URL 不通

- 运行 `./scripts/dev-tunnel.sh status`
- 确认 ngrok 仍在运行
- 如果看到 `ngrok public URL: unavailable`，重启 tunnel

### 8766 端口已被占用

先停掉已经绑定在 `NOTION_LOCAL_OPS_HOST:NOTION_LOCAL_OPS_PORT` 的旧进程，再重新启动。
