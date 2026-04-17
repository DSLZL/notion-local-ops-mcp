# Notion Showcase

中文版本：[notion-showcase.zh-CN.md](./notion-showcase.zh-CN.md)

This document shows an **optional extension scenario** built on top of `notion-local-ops-mcp`.

It is not the core project itself.

The showcase demonstrates one specific combination:

- **Notion AI** uses a page-level instruction page
- an **MCP Agent** handles actual local execution through MCP
- **Projects / Tasks** pages provide project-management and handoff context inside Notion

## Canonical Public Page

- [Public Notion instruction-page demo](https://ncp.notion.site/Agent-Start-Here-Template-10eb4da3979d8396861281ca608bc34e)

This same public page is the one users duplicate into their own workspace.

In this optional workflow, its real working role is not just "a public page". It becomes the **page-level instruction page for Notion AI** after duplication and binding in `Notion AI > Instructions`.

This showcase is based on three user-provided screenshots:

1. instruction-page / coordination-hub overview
2. task board + task detail + MCP Agent side panel
3. dedicated handoff / progress page inside the agent workspace

## Read This Showcase With The Right Mental Model

Keep these layers separate:

- **Notion AI** = page-level behavior layer
- **MCP Agent** = execution layer that uses this repo's tools
- **Projects / Tasks** = runtime coordination surfaces
- **local repo + local docs** = source of truth for implementation

This distinction is the whole point of the optional demo.

## Screenshot 1: Instruction Page As The Coordination Hub

This view shows the page that anchors the optional workflow:

- it gives the workspace one fixed entry point
- it states that code and local docs are the source of truth
- it defines the routing order `Task -> Project -> AGENTS.md`
- it defines short write-back fields such as status, verification summary, latest task id, and latest commit

What matters most here is the role of the page:

- the page is duplicated into the user's workspace
- then it is bound in `Notion AI > Instructions`
- that makes it the page-level instruction page for **Notion AI**

So this page should be understood first as an instruction-page surface, and only second as something that happens to be published publicly.

## Screenshot 2: Task Execution View

This is where the separation between **Notion AI** and the **MCP Agent** becomes easiest to understand:

- left: task queue / board
- center: the selected task with execution properties
- right: the MCP Agent conversation and execution notes

Why this matters:

- the user can start from a task instead of restating repo context every session
- the MCP Agent can derive the working directory from the related project
- the MCP Agent can execute local work through MCP
- short execution results can be written back into Notion without turning Notion into the implementation source of truth

The public instruction-page demo intentionally includes sample rows from this repo so the structure is immediately visible. After duplication, users should replace those rows with their own data.

## Screenshot 3: Dedicated Handoff / Progress Page

This view shows a stronger handoff pattern inside the optional workflow:

- a dedicated page is created for the current task handoff
- the page records task URL, project, branch, verification summary, and next objective
- the MCP Agent can treat it as a focused execution brief instead of depending on chat context alone

This is especially useful when:

- a task spans multiple sessions
- one MCP Agent hands off to another
- the user wants a compact "latest known state" page before the next coding pass

## Example User Flows

### 1. Core project only: use MCP without the Notion demo

Example request:

```text
Open my local repo, inspect README.md, and summarize the project.
```

Expected behavior:

- the MCP Agent searches the local repo first
- it reads the file through MCP, not from Notion pages
- it answers like a coding agent, not like a workspace wiki assistant

### 2. Optional demo: start from a task

Example request:

```text
Start from the task "[E-1] Planner integration", read the related project, and tell me which directory and docs you will use before you change code.
```

Expected behavior:

- the MCP Agent reads the current task first
- then reads the related project
- derives the working directory from `Project.Default CWD` or `CWD Override`
- reads `AGENTS.md` in that directory before making changes

### 3. Optional demo: write back execution results

Example request:

```text
Finish this task, run the relevant verification command, and write the latest commit and verification summary back to Notion.
```

Expected behavior:

- the MCP Agent does the local work through MCP
- the task status is updated to `In Progress`, `Blocked`, or `Done`
- fields such as `Latest Commit`, `Latest Verification`, and `Latest Local Task ID` are written back when relevant

## What This Optional Workflow Is Not

This workflow is not trying to make Notion the source of truth for implementation.

The intended split is:

- **local repo + local docs**: implementation source of truth
- **Notion AI instruction page**: page-level behavior layer
- **MCP Agent prompt**: execution behavior layer
- **Projects / Tasks**: coordination and short write-back

If you only want the MCP connection, skip this optional demo entirely and use the setup in [Notion setup guide](./notion-setup.md).
