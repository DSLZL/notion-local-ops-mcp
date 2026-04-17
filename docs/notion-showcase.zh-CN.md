# Notion 效果展示

English version: [notion-showcase.md](./notion-showcase.md)

这份文档展示的是一个**可选扩展示范场景**，它建立在 `notion-local-ops-mcp` 之上。

它不是这个项目本体。

这份 showcase 演示的是下面这组特定组合：

- **Notion AI**：负责页面级指令
- **MCP Agent**：通过 MCP 负责真实的本地执行
- **Projects / Tasks**：在 Notion 里提供项目管理和交接上下文

## 统一公开页面

- [公开 Notion 指令页示例](https://ncp.notion.site/Agent-Start-Here-Template-10eb4da3979d8396861281ca608bc34e)

用户 duplicate 到自己 workspace 的，就是这同一个公开页面。

在这套可选工作流里，它真正的工作角色不只是“公开页面”。它会在 duplicate 并绑定到 `Notion AI > 指令` 之后，成为 **Notion AI 的页面级指令页**。

这份 showcase 基于你提供的 3 张图：

1. 指令页 / Coordination Hub 总览
2. task board + task detail + MCP Agent 侧边面板
3. agent workspace 里的专门交接 / 进度页

## 先用正确的心智模型来读这份文档

请把下面几层分开理解：

- **Notion AI** = 页面级行为层
- **MCP Agent** = 使用本仓库工具的执行层
- **Projects / Tasks** = 运行时协调界面
- **本地 repo + 本地文档** = 实现层 source of truth

这正是这套可选示范想说明的重点。

## 图 1：指令页作为 Coordination Hub

这张图展示的是整套可选工作流的锚点页面：

- 给整个 workspace 一个固定入口
- 明确写清代码和本地文档才是 source of truth
- 定义 `Task -> Project -> AGENTS.md` 的路由顺序
- 定义 status、verification summary、latest task id、latest commit 等简短回写字段

这里最关键的是这张页面的角色：

- 用户先把它 duplicate 进自己的 workspace
- 再去 `Notion AI > 指令` 里绑定它
- 这样它就成为 **Notion AI 的页面级指令页**

所以这张页面首先应该被理解成“指令页表面”，其次才是“刚好被公开发布出来”。

## 图 2：任务执行视图

这一张最容易看出 **Notion AI** 和 **MCP Agent** 的区别：

- 左边：任务队列 / 看板
- 中间：当前 task 的详情和执行属性
- 右边：MCP Agent 的对话和执行记录

它的价值在于：

- 用户可以直接从 task 开始，而不是每次重复解释 repo 背景
- MCP Agent 可以从关联 project 推导正确工作目录
- MCP Agent 可以通过 MCP 执行真实的本地工作
- 执行结果可以简短回写到 Notion，但不会让 Notion 变成实现层 source of truth

这个公开指令页示例故意保留了本仓库的示例行数据，目的是让结构一眼可见。duplicate 后，用户应该尽快替换成自己的数据。

## 图 3：专门的交接 / 进度页

这张图展示的是这套可选工作流里更强的 handoff 模式：

- 为当前任务单独建立一页交接说明
- 页面里记录 task URL、project、branch、verification summary、下一个目标
- MCP Agent 可以把它当成聚焦的执行 brief，而不是只靠聊天上下文推测

它尤其适合这些情况：

- 一个任务跨多个 session
- 一个 MCP Agent 把上下文交接给另一个 MCP Agent
- 用户希望下一轮编码前，先有一份紧凑的“最新已知状态”页面

## 示例用户流程

### 1. 只使用核心项目：只接 MCP，不用 Notion 示范

示例请求：

```text
Open my local repo, inspect README.md, and summarize the project.
```

预期行为：

- MCP Agent 先搜索本地 repo
- 它通过 MCP 读取文件，而不是去 Notion 页面里找
- 最终回答更像 coding agent，而不是 workspace wiki assistant

### 2. 可选示范：从 task 开始

示例请求：

```text
Start from the task "[E-1] Planner integration", read the related project, and tell me which directory and docs you will use before you change code.
```

预期行为：

- MCP Agent 先读当前 task
- 再读关联 project
- 从 `Project.Default CWD` 或 `CWD Override` 推导工作目录
- 在真正改代码前读取该目录下的 `AGENTS.md`

### 3. 可选示范：回写执行结果

示例请求：

```text
Finish this task, run the relevant verification command, and write the latest commit and verification summary back to Notion.
```

预期行为：

- MCP Agent 通过 MCP 完成本地工作
- task status 被更新为 `In Progress`、`Blocked` 或 `Done`
- 视情况回写 `Latest Commit`、`Latest Verification`、`Latest Local Task ID`

## 这套可选工作流不是什么

这套工作流的目标，不是把 Notion 变成实现层的 source of truth。

正确边界应该是：

- **本地 repo + 本地文档**：实现层 source of truth
- **Notion AI 指令页**：页面级行为层
- **MCP Agent prompt**：执行行为层
- **Projects / Tasks**：协调和简短回写层

如果你只想使用 MCP 连接，完全可以跳过这套可选示范，直接看 [Notion 配置指南](./notion-setup.zh-CN.md)。
