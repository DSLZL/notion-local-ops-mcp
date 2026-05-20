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

## 长任务命令流

短命令、希望在一次 MCP 请求里直接拿到结果时，用 `run_command`。

耗时命令、需要轮询进度或日志时，用 `run_command_stream`。它会立刻返回 `task_id`，之后优先用 `wait_task`，不要高频忙轮询 `get_task`。

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

这个仓库现在可以直接打包成 Docker 容器运行，不再依赖 ngrok。镜像运行时已切换为 Kali，目标是把 MCP 服务放进受限的 CTF 工具容器里，而不是继续使用最小化 Alpine 基础镜像。为了把所有可写运行态统一收敛到一个临时区域，镜像仍然只复制 MCP 服务二进制，并以下列配置启动：

- `NOTION_LOCAL_OPS_WORKSPACE_ROOT=/tmp`
- `NOTION_LOCAL_OPS_STATE_DIR=/tmp/notion-local-ops-mcp`
- `HOME=/tmp`

附带的 [`docker-compose.yml`](./docker-compose.yml) 还会设置：

- `read_only: true`
- `tmpfs: /tmp`
- `cap_drop: [ALL]`
- `no-new-privileges:true`
- `pids_limit: 256`
- `mem_limit: 2g`
- `cpus: 2.0`

这意味着：

- 命令只会在容器内执行
- 不挂载宿主机目录
- 默认工作目录就是容器内独立的 `/tmp`
- 任务状态不持久化
- 重启或重建容器后，运行态会全部还原
- 继续使用 Docker 默认的 bridge 网络，而不是 host 网络
- 镜像内预装了 Kali headless 工具集以及 `nmap`、`gdb`、`ffuf`、`sqlmap`、`pwntools` 等常用 CTF 工具

构建并启动：

```bash
docker compose up --build -d
```

上线前请设置真实 Bearer Token：

```yaml
environment:
  NOTION_LOCAL_OPS_AUTH_TOKEN: your-strong-token
```

服务端现在也会强制工作区约束：解析出的路径和 session cwd 不能逃出 `/tmp`，即使是绝对路径也会被拒绝。

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
