# Notion 配置指南

English version: [notion-setup.md](./notion-setup.md)

这份指南会明确拆成两部分：

- **核心配置**：让 **MCP Agent** 接上 `notion-local-ops-mcp`
- **可选示范工作流**：在此基础上，再加上 Notion AI 指令页和 Projects / Tasks 项目管理页

后者只是扩展示例，不是本仓库的核心本体。

## 术语约定

为了避免把几件不同的事混在一起理解，本文统一使用这些名字：

1. **Notion AI**
   指 Notion 里的页面级 AI 行为层。在可选示范里，它会读取你在 `Notion AI > 指令` 里绑定的 duplicate 页面。
2. **MCP Agent**
   指通过 `notion-local-ops-mcp` 使用本地文件、shell、git 和 delegate 能力的 agent。
3. **指令页**
   指可选示范里 duplicate 后给 Notion AI 使用的公开页面。
4. **Projects / Tasks**
   指只在可选示范里使用的运行时协调页面。

## 1. 核心配置：先启动本地 MCP 服务

最快的方式：

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp
cp .env.example .env
```

编辑 `.env`，至少填这两个值：

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

然后运行：

```bash
./scripts/dev-tunnel.sh
```

你应该会得到：

- 本地 MCP 服务：`http://127.0.0.1:8766/mcp`
- `cloudflared` 给出的公网 tunnel 地址

在 Notion 里配置你的 MCP Agent 时，使用这个地址并在后面加上 `/mcp`。

## 2. 核心配置：配置 MCP Agent

在 Notion 里的 MCP Agent 配置中，添加一个自定义 MCP 连接：

- URL：`https://<your-domain-or-tunnel>/mcp`
- Auth type：`Bearer`
- Token：本地 `.env` 里的 `NOTION_LOCAL_OPS_AUTH_TOKEN`

如果 Notion 提示连接失败，优先检查：

- URL 是否以 `/mcp` 结尾
- 鉴权类型是否是 `Bearer`
- token 是否和本地 `.env` 一致
- `cloudflared` 是否还在运行

## 3. 核心配置：给 MCP Agent 粘贴 Prompt

直接使用主 [README](../README.zh-CN.md) 里的 prompt。

这套 prompt 的目标，是让 **MCP Agent** 更像本地 coding agent，而不是 Notion 页面编辑器：

- 优先搜索本地 repo
- 除非用户明确说的是 Notion 页面，否则默认把 repo 文档当本地文件处理
- 先用直接工具，再考虑 delegate
- 修改后尽量运行命令或测试做验证

但要注意：这份 prompt **不能替代** Notion AI 的页面级指令页。两者职责不同。

## 4. 可选示范：复制公开 Notion 指令页

如果你想使用可选的项目管理工作流，就把这个公开页面 duplicate 到你自己的 workspace：

- [公开 Notion 指令页示例](https://ncp.notion.site/Agent-Start-Here-Template-10eb4da3979d8396861281ca608bc34e)

请优先把它理解成一个**指令页示例**：

- 它被公开发布，方便你先预览
- 你 duplicate 的就是这同一页
- 在可选工作流里，它真正的角色是成为 **Notion AI 的页面级指令页**

duplicate 后页面里会带有：

- `Agent Start Here`：稳定规则入口页
- `Projects`：repo 级默认配置，例如 workspace root 和默认工作目录
- `Tasks`：任务状态、回写字段和执行上下文

这个公开页面故意保留了本仓库的示例行数据，目的是让结构一眼就能看懂。duplicate 之后建议立刻：

1. 把示例 project 改成你自己的 repo
2. 删除或改写示例 task
3. 填好自己的 `Workspace Root`、`Default CWD`、`Local Docs Root`
4. 去 `Notion AI > 指令`，把 duplicate 出来的这一页设成 Notion AI 的指令页

第 4 步是关键。没有这一步，这页只是你 workspace 里的一张复制页，还不是正在生效的 Notion AI 指令页。

## 5. 第一次验证建议

MCP 连接成功后，建议做下面几个最小检查。

### 检查 A：MCP Agent 能读本地文件

例如：

```text
Read the local README.md in my repo and summarize what this project does.
```

### 检查 B：MCP Agent 能执行无害 shell 命令

例如：

```text
Run pwd and tell me which working directory you are using.
```

### 检查 C：可选示范能从 Task 路由到 Project

只有在你 duplicate 了公开指令页示例之后，才做这一步。

例如：

```text
Open the task "Design Notion project and task system", read its linked project, and explain which local directory you would use.
```

## 6. 什么算配置成功

配置完成后：

### 只使用核心项目时

你的 MCP Agent 应该可以：

- 读取本地 repo 文件，而不是误把它当成 Notion 页面
- 通过 MCP 执行 shell 命令
- 以本地工具优先的方式完成 coding 风格的仓库操作

### 使用可选的 Notion AI 示范工作流时

你还会额外得到：

- 一张给 Notion AI 使用的页面级指令页
- 从 `Task -> Project -> AGENTS.md` 的任务路由
- 向 Notion 回写简短执行状态
- 一个可复用的 Notion 项目管理表面，但不会让 Notion 变成实现层 source of truth

如果你想看这套可选工作流的实际样子，见 [Notion 效果展示](./notion-showcase.zh-CN.md)。
