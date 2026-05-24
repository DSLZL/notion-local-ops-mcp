package mcp

import (
	"encoding/json"
	"fmt"
	"os"

	"notion-local-ops-mcp-go/internal/config"
	"notion-local-ops-mcp-go/internal/tools"
)

func CallTool(cfg config.Config, name string, params map[string]any) (map[string]any, error) {
	workspaceRoot := cfg.WorkspaceRoot
	if workspaceRoot == "" {
		workspaceRoot = "."
	}
	stateDir := cfg.StateDir
	if stateDir == "" {
		stateDir = ".notion-local-ops-mcp"
	}

	switch name {
	case "server_info":
		return toolSuccessResult(tools.BuildServerInfo(cfg, toolNames()))
	case "list_skills":
		includeProject := true
		if value, ok := boolParam(params, "include_project"); ok {
			includeProject = value
		}
		includeGlobal := true
		if value, ok := boolParam(params, "include_global"); ok {
			includeGlobal = value
		}
		namespace, _ := stringParam(params, "namespace")
		namePattern, _ := stringParam(params, "name_pattern")
		descriptionMaxLength, _ := intParam(params, "description_max_length")
		return toolSuccessResult(tools.ListSkills(workspaceRoot, namespace, namePattern, descriptionMaxLength, includeProject, includeGlobal))
	case "set_default_cwd":
		path, _ := stringParam(params, "path")
		if path == "" {
			tools.ClearDefaultCWD()
			return toolSuccessResult(map[string]any{
				"success":        true,
				"session_cwd":    nil,
				"workspace_root": workspaceRoot,
				"cleared":        true,
			})
		}
		resolved, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("working directory not found: %s", path)
		}
		if !resolved.IsDir() {
			return nil, fmt.Errorf("working directory is not a directory: %s", path)
		}
		absPath, _, err := tools.ResolveSessionDirectory(workspaceRoot, path)
		if err != nil {
			return nil, err
		}
		tools.SetDefaultCWD(absPath)
		return toolSuccessResult(map[string]any{
			"success":        true,
			"session_cwd":    absPath,
			"workspace_root": workspaceRoot,
			"cleared":        false,
		})
	case "get_default_cwd":
		sessionCWD := tools.GetDefaultCWD()
		source := "workspace_root"
		effective := workspaceRoot
		var current any
		if sessionCWD != "" {
			source = "session"
			effective = sessionCWD
			current = sessionCWD
		}
		return toolSuccessResult(map[string]any{
			"success":        true,
			"session_cwd":    current,
			"workspace_root": workspaceRoot,
			"effective_cwd":  effective,
			"source":         source,
		})
	case "list_files":
		path, _ := stringParam(params, "path")
		recursive, _ := boolParam(params, "recursive")
		limit, _ := intParam(params, "limit")
		offset, _ := intParam(params, "offset")
		includeHidden, _ := boolParam(params, "include_hidden")
		entries, err := tools.ListFiles(workspaceRoot, tools.ListFilesOptions{
			Path:          path,
			Recursive:     recursive,
			Limit:         limit,
			Offset:        offset,
			IncludeHidden: includeHidden,
		})
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(entries)
	case "read_text":
		path, err := requiredStringParam(params, "path")
		if err != nil {
			return nil, err
		}
		startLine, _ := intParam(params, "start_line")
		lineLimit, _ := intParam(params, "line_limit")
		includeLineNumbers, _ := boolParam(params, "include_line_numbers")
		text, err := tools.ReadText(workspaceRoot, tools.ReadTextOptions{
			Path:               path,
			StartLine:          startLine,
			LineLimit:          lineLimit,
			IncludeLineNumbers: includeLineNumbers,
		})
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(text)
	case "write_file":
		path, err := requiredStringParam(params, "path")
		if err != nil {
			return nil, err
		}
		dryRun, _ := boolParam(params, "dry_run")
		content, hasContent := stringParam(params, "content")
		contentBase64, hasContentBase64 := stringParam(params, "content_base64")
		return toolSuccessResult(mustWriteFileWithOptions(workspaceRoot, tools.WriteFileOptions{
			Path:          path,
			Content:       optionalStringPtr(content, hasContent),
			ContentBase64: optionalStringPtr(contentBase64, hasContentBase64),
			DryRun:        dryRun,
		}, cfg.ExtraWriteDirs))
	case "search":
		mode, _ := stringParam(params, "mode")
		path, _ := stringParam(params, "path")
		pattern, _ := stringParam(params, "pattern")
		query, _ := stringParam(params, "query")
		before, _ := intParam(params, "before")
		after, _ := intParam(params, "after")
		limit, _ := intParam(params, "limit")
		if mode == "" && query != "" {
			mode = "text"
		}
		if mode == "" {
			mode = "regex"
		}
		matches, err := tools.Search(workspaceRoot, tools.SearchOptions{
			Mode:    mode,
			Path:    path,
			Pattern: pattern,
			Query:   query,
			Before:  before,
			After:   after,
			Limit:   limit,
		})
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(matches)
	case "run_command":
		commandText, err := requiredStringParam(params, "command")
		if err != nil {
			return nil, err
		}
		cwd, _ := stringParam(params, "cwd")
		runInBackground, _ := boolParam(params, "run_in_background")
		timeout, _ := intParam(params, "timeout")
		stdinContent := firstStringParam(params, "stdin", "stdin_content")
		if runInBackground {
			return toolSuccessResult(tools.RunCommandStream(stateDir, workspaceRoot, commandText, cwd, timeout))
		}
		return toolSuccessResult(tools.RunCommand(workspaceRoot, commandText, cwd, stdinContent, timeout))
	case "apply_patch":
		patch, err := requiredStringParam(params, "patch")
		if err != nil {
			return nil, err
		}
		dryRun, _ := boolParam(params, "dry_run")
		validateOnly, _ := boolParam(params, "validate_only")
		returnDiff, _ := boolParam(params, "return_diff")
		return toolSuccessResult(tools.ApplyPatch(workspaceRoot, patch, dryRun, validateOnly, returnDiff))
	case "run_command_stream":
		commandText, _ := stringParam(params, "command")
		if commandText == "" {
			commandText = "echo stream-ok"
		}
		cwd, _ := stringParam(params, "cwd")
		timeout, _ := intParam(params, "timeout")
		return toolSuccessResult(tools.RunCommandStream(stateDir, workspaceRoot, commandText, cwd, timeout))
	case "wait_task":
		taskID, err := requiredStringParam(params, "task_id")
		if err != nil {
			return nil, err
		}
		timeoutSeconds, _ := intParam(params, "timeout_seconds")
		lastEventSeq, _ := intParam(params, "last_event_seq")
		return toolSuccessResult(tools.WaitTask(stateDir, taskID, timeoutSeconds, int64(lastEventSeq)))
	case "await_task":
		taskID, err := requiredStringParam(params, "task_id")
		if err != nil {
			return nil, err
		}
		maxWaitSeconds, _ := intParam(params, "max_wait_seconds")
		lastEventSeq, _ := intParam(params, "last_event_seq")
		includeLogs, _ := boolParam(params, "include_logs")
		logStream, _ := stringParam(params, "log_stream")
		if logStream == "" {
			logStream = "stdout"
		}
		logLimit, _ := intParam(params, "log_limit")
		return toolSuccessResult(tools.AwaitTask(stateDir, taskID, maxWaitSeconds, int64(lastEventSeq), includeLogs, logStream, int64(logLimit)))
	case "get_task":
		taskID, err := requiredStringParam(params, "task_id")
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(tools.GetTask(stateDir, taskID))
	case "list_recent_tasks":
		status, _ := stringParam(params, "status")
		limit, _ := intParam(params, "limit")
		return toolSuccessResult(tools.ListRecentTasks(stateDir, status, limit))
	case "get_task_logs":
		taskID, err := requiredStringParam(params, "task_id")
		if err != nil {
			return nil, err
		}
		stream, err := requiredStringParam(params, "stream")
		if err != nil {
			return nil, err
		}
		offset, _ := intParam(params, "offset")
		limit, _ := intParam(params, "limit")
		return toolSuccessResult(tools.GetTaskLogs(stateDir, taskID, stream, int64(offset), int64(limit)))
	case "cancel_task":
		taskID, err := requiredStringParam(params, "task_id")
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(tools.CancelTask(stateDir, taskID))
	case "purge_tasks":
		olderThanHours, ok := intParam(params, "older_than_hours")
		if !ok || olderThanHours <= 0 {
			olderThanHours = 24 * 7
		}
		dryRun, _ := boolParam(params, "dry_run")
		result, err := tools.PurgeTasks(stateDir, olderThanHours, dryRun)
		if err != nil {
			return nil, err
		}
		return toolSuccessResult(result)
	case "git_status":
		return toolSuccessResult(tools.GitStatus(tools.EffectiveCWD(workspaceRoot)))
	case "git_diff":
		staged, _ := boolParam(params, "staged")
		paths := stringSliceParam(params, "paths")
		maxBytes, ok := intParam(params, "max_bytes")
		if !ok || maxBytes <= 0 {
			maxBytes = 65536
		}
		perFileMaxBytes, ok := intParam(params, "per_file_max_bytes")
		if !ok || perFileMaxBytes <= 0 {
			perFileMaxBytes = 16384
		}
		return toolSuccessResult(tools.GitDiff(tools.EffectiveCWD(workspaceRoot), staged, paths, maxBytes, perFileMaxBytes))
	case "git_commit":
		message, err := requiredStringParam(params, "message")
		if err != nil {
			return nil, err
		}
		paths := stringSliceParam(params, "paths")
		stageAll, _ := boolParam(params, "stage_all")
		amend, _ := boolParam(params, "amend")
		allowEmpty, _ := boolParam(params, "allow_empty")
		author, _ := stringParam(params, "author")
		signOff, _ := boolParam(params, "sign_off")
		dryRun, _ := boolParam(params, "dry_run")
		return toolSuccessResult(tools.GitCommit(tools.EffectiveCWD(workspaceRoot), message, paths, stageAll, amend, allowEmpty, author, signOff, dryRun))
	case "git_log":
		limit, ok := intParam(params, "limit")
		if !ok || limit <= 0 {
			limit = 10
		}
		return toolSuccessResult(tools.GitLog(tools.EffectiveCWD(workspaceRoot), limit))
	case "git_show":
		ref, _ := stringParam(params, "ref")
		maxBytes, ok := intParam(params, "max_bytes")
		if !ok || maxBytes <= 0 {
			maxBytes = 65536
		}
		perFileMaxBytes, ok := intParam(params, "per_file_max_bytes")
		if !ok || perFileMaxBytes <= 0 {
			perFileMaxBytes = 16384
		}
		return toolSuccessResult(tools.GitShow(tools.EffectiveCWD(workspaceRoot), ref, maxBytes, perFileMaxBytes))
	case "git_blame":
		path, err := requiredStringParam(params, "path")
		if err != nil {
			return nil, err
		}
		ref, _ := stringParam(params, "ref")
		startLine, _ := optionalIntParam(params, "start_line")
		endLine, _ := optionalIntParam(params, "end_line")
		return toolSuccessResult(tools.GitBlame(tools.EffectiveCWD(workspaceRoot), path, ref, startLine, endLine))
	default:
		return nil, fmt.Errorf("unknown tool")
	}
}

func toolNames() []string {
	toolsList := CoreTools()
	names := make([]string, 0, len(toolsList))
	for _, tool := range toolsList {
		names = append(names, tool.Name)
	}
	return names
}

func mustWriteFile(workspaceRoot, path, content string, dryRun bool, extraWriteDirs []string) tools.WriteFileResult {
	result, err := tools.WriteFile(workspaceRoot, path, content, dryRun, extraWriteDirs)
	if err != nil {
		return tools.WriteFileResult{
			Success: false,
			Path:    path,
			DryRun:  dryRun,
			Summary: err.Error(),
		}
	}
	return result
}

func mustWriteFileWithOptions(workspaceRoot string, options tools.WriteFileOptions, extraWriteDirs []string) tools.WriteFileResult {
	result, err := tools.WriteFileWithOptions(workspaceRoot, options, extraWriteDirs)
	if err != nil {
		return tools.WriteFileResult{
			Success: false,
			Path:    options.Path,
			DryRun:  options.DryRun,
			Summary: err.Error(),
		}
	}
	return result
}

func toolSuccessResult(payload any) (map[string]any, error) {
	rendered, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": string(rendered),
			},
		},
		"structuredContent": payload,
		"isError":           false,
	}, nil
}

func requiredStringParam(params map[string]any, key string) (string, error) {
	value, ok := stringParam(params, key)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func stringParam(params map[string]any, key string) (string, bool) {
	if params == nil {
		return "", false
	}
	value, ok := params[key].(string)
	if !ok {
		return "", false
	}
	return value, true
}

func firstStringParam(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := stringParam(params, key); ok {
			return value
		}
	}
	return ""
}

func optionalStringPtr(value string, ok bool) *string {
	if !ok {
		return nil
	}
	return &value
}

func boolParam(params map[string]any, key string) (bool, bool) {
	if params == nil {
		return false, false
	}
	value, ok := params[key].(bool)
	if !ok {
		return false, false
	}
	return value, true
}

func intParam(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	switch value := params[key].(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	default:
		return 0, false
	}
}

func optionalIntParam(params map[string]any, key string) (*int, bool) {
	value, ok := intParam(params, key)
	if !ok {
		return nil, false
	}
	return &value, true
}

func stringSliceParam(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if ok && text != "" {
			values = append(values, text)
		}
	}
	return values
}
