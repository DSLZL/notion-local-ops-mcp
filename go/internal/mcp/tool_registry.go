package mcp

import "sort"

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

var coreTools = []Tool{
	{
		Name:        "server_info",
		Description: "Inspect runtime config and available MCP tools.",
		InputSchema: objectSchema(),
	},
	{
		Name:        "list_skills",
		Description: "List project and global agent skills as lightweight summaries.",
		InputSchema: objectSchema(
			boolProperty("include_project", "When true, include project skill roots."),
			boolProperty("include_global", "When true, include global skill roots."),
			stringProperty("namespace", "Optional skill namespace filter such as agents, codex, or claude."),
			stringProperty("name_pattern", "Optional fnmatch-style pattern for skill names."),
			intProperty("description_max_length", "Optional description length cap."),
		),
	},
	{
		Name:        "set_default_cwd",
		Description: "Set or clear the session default working directory.",
		InputSchema: objectSchema(
			stringProperty("path", "Optional absolute or relative directory path to store as the session cwd. Empty clears it."),
		),
	},
	{
		Name:        "get_default_cwd",
		Description: "Return the active default working directory and its source.",
		InputSchema: objectSchema(),
	},
	{
		Name:        "list_files",
		Description: "List directory contents with optional recursion, pagination, and hidden-file control.",
		InputSchema: objectSchema(
			stringProperty("path", "Relative directory path inside the workspace root."),
			boolProperty("recursive", "When true, recurse into subdirectories."),
			intProperty("limit", "Maximum number of entries to return."),
			intProperty("offset", "Entry offset for pagination."),
			boolProperty("include_hidden", "When true, include dotfiles and dot-directories."),
		),
	},
	{
		Name:        "read_text",
		Description: "Read a UTF-8 text file with line-based pagination and optional line numbers.",
		InputSchema: requiredObjectSchema([]string{"path"},
			stringProperty("path", "Relative file path inside the workspace root."),
			intProperty("start_line", "Optional 1-based start line."),
			intProperty("line_limit", "Optional number of lines to return."),
			boolProperty("include_line_numbers", "When true, prefix each line with its line number."),
		),
	},
	{
		Name:        "write_file",
		Description: "Write full content to a file with optional dry-run validation. Provide exactly one of content or content_base64.",
		InputSchema: requiredObjectSchema([]string{"path"},
			stringProperty("path", "Relative file path inside the workspace root."),
			stringProperty("content", "Full text content to write."),
			stringProperty("content_base64", "Optional base64-encoded bytes to decode server-side and write byte-for-byte."),
			boolProperty("dry_run", "When true, validate without touching disk."),
		),
	},
	{
		Name:        "search",
		Description: "Search workspace files in glob, regex, or text mode with optional context lines.",
		InputSchema: objectSchema(
			stringProperty("mode", "Search mode: glob, regex, or text."),
			stringProperty("path", "Optional relative file or directory path to narrow the search scope."),
			stringProperty("pattern", "Glob or regex pattern depending on mode."),
			stringProperty("query", "Literal text query for text mode."),
			intProperty("before", "Context lines to include before each match."),
			intProperty("after", "Context lines to include after each match."),
			intProperty("limit", "Maximum number of matches to return."),
		),
	},
	{
		Name:        "apply_patch",
		Description: "Apply a structured patch using *** Begin Patch / *** Update File blocks with optional dry-run, validation-only, and diff preview modes.",
		InputSchema: requiredObjectSchema([]string{"patch"},
			stringProperty("patch", "Structured patch text in *** Begin Patch format."),
			boolProperty("dry_run", "When true, validate and preview changes without writing to disk."),
			boolProperty("validate_only", "When true, validate patch semantics without writing to disk."),
			boolProperty("return_diff", "When true, include a rendered diff preview in the response."),
		),
	},
	{
		Name:        "run_command",
		Description: "Run a local shell command now or queue it as a background task for wait_task/get_task polling.",
		InputSchema: requiredObjectSchema([]string{"command"},
			stringProperty("command", "Shell command to execute."),
			stringProperty("cwd", "Optional relative working directory inside the workspace root."),
			intProperty("timeout", "Optional timeout in seconds for foreground execution."),
			boolProperty("run_in_background", "When true, queue the command as a pollable background task."),
			stringProperty("stdin", "Optional text to feed to the command via standard input. Avoids heredoc escaping issues."),
			stringProperty("stdin_content", "Alias of stdin for clients that prefer an explicit request-side content field name."),
		),
	},
	{
		Name:        "run_command_stream",
		Description: "Run a shell command, persist its result under STATE_DIR/tasks, and return a pollable task id.",
		InputSchema: objectSchema(
			stringProperty("command", "Shell command to execute. Defaults to a small echo smoke command when omitted."),
			stringProperty("cwd", "Optional relative working directory inside the workspace root."),
		),
	},
	{
		Name:        "wait_task",
		Description: "Long-poll a background task until it reaches a terminal state or a newer event_seq is available.",
		InputSchema: requiredObjectSchema(
			[]string{"task_id"},
			stringProperty("task_id", "Task id returned by run_command_stream."),
			intProperty("timeout_seconds", "Optional long-poll timeout in seconds. Defaults to 1."),
			intProperty("last_event_seq", "Optional last seen event_seq. When the task advances beyond this value, wait_task returns immediately."),
		),
	},
	{
		Name:        "await_task",
		Description: "Wait for a background task across multiple poll cycles and optionally include task logs for resume-friendly clients.",
		InputSchema: requiredObjectSchema(
			[]string{"task_id"},
			stringProperty("task_id", "Task id returned by run_command_stream."),
			intProperty("max_wait_seconds", "Optional maximum total wait time in seconds across repeated polling. Defaults inside the tool when omitted."),
			intProperty("last_event_seq", "Optional last seen event_seq to resume from without replaying unchanged state."),
			boolProperty("include_logs", "When true, include a log excerpt in the response."),
			stringProperty("log_stream", "Optional log stream to read when include_logs is true. Defaults to stdout when omitted."),
			intProperty("log_limit", "Optional maximum log bytes to include when include_logs is true."),
		),
	},
	{
		Name:        "get_task",
		Description: "Fetch the latest rich polling state for a background task, including event_seq, progress, heartbeat, and next poll guidance.",
		InputSchema: requiredObjectSchema([]string{"task_id"}, stringProperty("task_id", "Task id returned by run_command_stream.")),
	},
	{
		Name:        "list_recent_tasks",
		Description: "List recent persisted background tasks with optional status filtering and limit control for resume flows.",
		InputSchema: objectSchema(
			stringProperty("status", "Optional task status filter such as running, succeeded, failed, or canceled."),
			intProperty("limit", "Optional maximum number of recent tasks to return."),
		),
	},
	{
		Name:        "get_task_logs",
		Description: "Read stdout or stderr logs for a background task incrementally using offset and limit.",
		InputSchema: requiredObjectSchema([]string{"task_id", "stream"},
			stringProperty("task_id", "Task id returned by run_command_stream."),
			stringProperty("stream", "Log stream to read: stdout or stderr."),
			intProperty("offset", "Optional byte offset to start reading from."),
			intProperty("limit", "Optional maximum byte count to return."),
		),
	},
	{
		Name:        "cancel_task",
		Description: "Request cancellation for a running background task and return the latest known task state.",
		InputSchema: requiredObjectSchema([]string{"task_id"}, stringProperty("task_id", "Task id returned by run_command_stream.")),
	},
	{
		Name:        "purge_tasks",
		Description: "Remove old task records from STATE_DIR/tasks with dry-run support.",
		InputSchema: objectSchema(
			intProperty("older_than_hours", "Purge tasks older than this many hours. Defaults to 168."),
			boolProperty("dry_run", "When true, preview which task directories would be purged."),
		),
	},
	{
		Name:        "git_status",
		Description: "Return structured git status for the repository at the current workspace root.",
		InputSchema: objectSchema(),
	},
	{
		Name:        "git_diff",
		Description: "Return git diff output plus per-file diffs with added/removed counts.",
		InputSchema: objectSchema(
			boolProperty("staged", "When true, read staged changes via git diff --cached."),
			stringArrayProperty("paths", "Optional pathspec list to restrict the diff."),
			intProperty("max_bytes", "Maximum bytes to keep in the combined diff field."),
			intProperty("per_file_max_bytes", "Maximum bytes to keep per file diff chunk."),
		),
	},
	{
		Name:        "git_commit",
		Description: "Stage selected paths or all changes and create a git commit, with amend, allow-empty, custom author, sign-off, and dry-run support.",
		InputSchema: requiredObjectSchema([]string{"message"},
			stringProperty("message", "Commit message summary."),
			stringArrayProperty("paths", "Optional pathspec list to stage before commit."),
			boolProperty("stage_all", "When true, stage all tracked and untracked changes via git add -A."),
			boolProperty("amend", "When true, amend the current HEAD commit."),
			boolProperty("allow_empty", "When true, allow creating an empty commit."),
			stringProperty("author", "Optional git author string like Name <email>."),
			boolProperty("sign_off", "When true, append Signed-off-by trailer."),
			boolProperty("dry_run", "When true, preview the commit plan without creating a commit."),
		),
	},
	{
		Name:        "git_log",
		Description: "Return recent git commits for the repository at the current workspace root.",
		InputSchema: objectSchema(
			intProperty("limit", "Maximum number of recent commits to return."),
		),
	},
	{
		Name:        "git_show",
		Description: "Show metadata plus per-file diff for a commit or git ref.",
		InputSchema: objectSchema(
			stringProperty("ref", "Git ref to inspect. Defaults to HEAD."),
			intProperty("max_bytes", "Maximum bytes to keep in the combined diff field."),
			intProperty("per_file_max_bytes", "Maximum bytes to keep per file diff chunk."),
		),
	},
	{
		Name:        "git_blame",
		Description: "Return per-line blame info for a file, optionally restricted to a line range.",
		InputSchema: requiredObjectSchema([]string{"path"},
			stringProperty("path", "Repository-relative file path to blame."),
			stringProperty("ref", "Optional git ref to blame against."),
			intProperty("start_line", "Optional inclusive start line."),
			intProperty("end_line", "Optional inclusive end line."),
		),
	},
}

func CoreTools() []Tool {
	tools := make([]Tool, len(coreTools))
	copy(tools, coreTools)
	return tools
}

func HasTool(name string) bool {
	for _, tool := range coreTools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func ToolByName(name string) (Tool, bool) {
	for _, tool := range coreTools {
		if tool.Name == name {
			return tool, true
		}
	}
	return Tool{}, false
}

func objectSchema(properties ...schemaProperty) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	if len(properties) == 0 {
		return schema
	}
	props := schema["properties"].(map[string]any)
	for _, property := range properties {
		props[property.Name] = property.Definition
	}
	return schema
}

func requiredObjectSchema(required []string, properties ...schemaProperty) map[string]any {
	schema := objectSchema(properties...)
	names := append([]string(nil), required...)
	sort.Strings(names)
	schema["required"] = names
	return schema
}

type schemaProperty struct {
	Name       string
	Definition map[string]any
}

func stringProperty(name, description string) schemaProperty {
	return schemaProperty{
		Name: name,
		Definition: map[string]any{
			"type":        "string",
			"description": description,
		},
	}
}

func intProperty(name, description string) schemaProperty {
	return schemaProperty{
		Name: name,
		Definition: map[string]any{
			"type":        "integer",
			"description": description,
		},
	}
}

func boolProperty(name, description string) schemaProperty {
	return schemaProperty{
		Name: name,
		Definition: map[string]any{
			"type":        "boolean",
			"description": description,
		},
	}
}

func stringArrayProperty(name, description string) schemaProperty {
	return schemaProperty{
		Name: name,
		Definition: map[string]any{
			"type":        "array",
			"description": description,
			"items": map[string]any{
				"type": "string",
			},
		},
	}
}
