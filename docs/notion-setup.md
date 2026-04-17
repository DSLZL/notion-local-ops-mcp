# Notion Setup Guide

中文版本：[notion-setup.zh-CN.md](./notion-setup.zh-CN.md)

This guide is intentionally split into two parts:

- the **core setup** for using `notion-local-ops-mcp` with an **MCP Agent**
- the **optional Notion AI demo workflow** that adds an instruction page plus Projects / Tasks pages for project management

The optional workflow is only an extension example. It is not the core of this repository.

## Terminology

To avoid mixing different concepts, this guide uses these names consistently:

1. **Notion AI**
   The page-level AI layer inside Notion. In the optional demo, it reads the duplicated page selected in `Notion AI > Instructions`.
2. **MCP Agent**
   The agent that uses `notion-local-ops-mcp` to access local files, shell, git, and delegated local coding tasks.
3. **Instruction page**
   The duplicated public page used by Notion AI in the optional demo.
4. **Projects / Tasks**
   The runtime coordination pages used only in the optional demo.

## 1. Core Setup: Start The Local MCP Server

The fastest path is:

```bash
git clone https://github.com/<your-account>/notion-local-ops-mcp.git
cd notion-local-ops-mcp
cp .env.example .env
```

Edit `.env` and set at least:

```bash
NOTION_LOCAL_OPS_WORKSPACE_ROOT="/absolute/path/to/workspace"
NOTION_LOCAL_OPS_AUTH_TOKEN="replace-me"
```

Then run:

```bash
./scripts/dev-tunnel.sh
```

You should get:

- a local MCP server at `http://127.0.0.1:8766/mcp`
- a public tunnel URL from `cloudflared`

Use the tunnel URL with `/mcp` appended when configuring your MCP Agent.

## 2. Core Setup: Configure The MCP Agent

In your MCP Agent configuration inside Notion, add a custom MCP connection with:

- URL: `https://<your-domain-or-tunnel>/mcp`
- Auth type: `Bearer`
- Token: the value of `NOTION_LOCAL_OPS_AUTH_TOKEN`

If Notion reports that it cannot connect:

- confirm the URL ends with `/mcp`
- confirm the auth type is `Bearer`
- confirm the token matches your local `.env`
- confirm `cloudflared` is still running

## 3. Core Setup: Paste The Prompt For The MCP Agent

Use the prompt in the main [README](../README.md).

That prompt is tuned to make the **MCP Agent** behave like a local coding agent rather than a Notion page editor:

- search the local repo first
- treat repo docs as local files unless the user explicitly says Notion page
- prefer direct local tools before delegation
- verify changes with commands or tests when possible

This prompt does **not** replace the Notion AI instruction page. They serve different roles.

## 4. Optional Demo: Duplicate The Public Notion Instruction Page

If you want the optional project-management workflow, duplicate this public page into your own workspace:

- [Public Notion instruction-page demo](https://ncp.notion.site/Agent-Start-Here-Template-10eb4da3979d8396861281ca608bc34e)

Treat this page first as an **instruction-page demo**:

- it is published publicly so you can preview it
- you duplicate that same page into your own workspace
- in the optional workflow, its working role is to become the page-level instruction page for **Notion AI**

The duplicated page includes:

- `Agent Start Here`: the stable rules page
- `Projects`: repo-level defaults such as workspace root and default working directory
- `Tasks`: task status, write-back fields, and execution context

That public page is intentionally seeded with example rows from this repo so the structure is visible immediately. After duplicating it:

1. replace the sample project rows with your own repo entries
2. delete or rewrite the sample task rows
3. set `Workspace Root`, `Default CWD`, and `Local Docs Root` for your repo
4. go to `Notion AI > Instructions` and set this duplicated page as the instruction page for Notion AI

That last step is essential. Without it, the page is only a copied template in your workspace, not an active instruction page for Notion AI.

## 5. First-Run Checks

Once the MCP connection is live, do these quick checks.

### Check A: MCP Agent can read a local file

Example:

```text
Read the local README.md in my repo and summarize what this project does.
```

### Check B: MCP Agent can run a harmless shell command

Example:

```text
Run pwd and tell me which working directory you are using.
```

### Check C: Optional demo workflow can route from Task to Project

Only do this if you duplicated the public instruction-page demo.

Example:

```text
Open the task "Design Notion project and task system", read its linked project, and explain which local directory you would use.
```

## 6. What Good Looks Like

After setup:

### Core project only

Your MCP Agent should be able to:

- inspect local repo files without treating them as Notion pages
- run shell commands through MCP
- perform coding-style repo work with local-tool-first behavior

### Optional Notion AI demo workflow

If you also use the optional demo, you should additionally get:

- a page-level instruction page for Notion AI
- task routing from `Task -> Project -> AGENTS.md`
- write-back of short execution state into Notion
- a reusable project-management surface inside Notion without turning Notion into the implementation source of truth

For an example of how that optional workflow looks in practice, see [Notion showcase](./notion-showcase.md).
