package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"notion-local-ops-mcp-go/internal/fsx"
	"notion-local-ops-mcp-go/internal/process"
	"notion-local-ops-mcp-go/internal/taskrunner"
	"notion-local-ops-mcp-go/internal/taskstore"
)

type TaskPollingResult struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

const TimeoutExitCode = -1

type RunCommandResult struct {
	Success  bool           `json:"success"`
	Command  string         `json:"command"`
	CWD      string         `json:"cwd"`
	ExitCode int            `json:"exit_code"`
	Stdout   string         `json:"stdout"`
	Stderr   string         `json:"stderr"`
	TimedOut bool           `json:"timed_out"`
	Timeout  int            `json:"timeout,omitempty"`
	Error    map[string]any `json:"error,omitempty"`
	Hint     string         `json:"hint,omitempty"`
}

func RunCommandStream(stateDir, workspace, commandText, cwd string, timeoutSeconds int) TaskPollingResult {
	root := stateDir
	if root == "" {
		root = taskPollingRoot()
	}
	if workspace == "" {
		workspace = "."
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}

	store := taskstore.NewFSStore(root)
	runner := taskrunner.NewRunner(store)
	created, err := runner.Submit(taskrunner.SubmitOptions{
		Task: taskstore.TaskInput{
			Task:     commandText,
			Executor: "run_command_stream",
			CWD:      cwd,
			Timeout:  timeoutSeconds,
		},
		Execute: func(ctx context.Context, onStart taskrunner.StartCallback) (process.Result, error) {
			return runStreamCommand(ctx, workspace, commandText, cwd, timeoutSeconds, onStart)
		},
	})
	if err != nil {
		return TaskPollingResult{Status: "failed", Summary: err.Error()}
	}

	return TaskPollingResult{
		TaskID:  created.ID,
		Status:  created.Status,
		Summary: "task accepted",
	}
}

func RunCommand(workspace, commandText, cwd string, timeoutSeconds int) RunCommandResult {
	resolvedCWD, err := resolveCommandCWD(workspace, cwd)
	if err != nil {
		return commandPathErrorResult(commandText, resolvedCWD, "cwd_not_found", fmt.Sprintf("Working directory not found: %s", resolvedCWD))
	}

	info, err := os.Stat(resolvedCWD)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return commandPathErrorResult(commandText, resolvedCWD, "cwd_not_found", fmt.Sprintf("Working directory not found: %s", resolvedCWD))
		}
		return commandPathErrorResult(commandText, resolvedCWD, "cwd_not_found", fmt.Sprintf("Working directory not found: %s", resolvedCWD))
	}
	if !info.IsDir() {
		return commandPathErrorResult(commandText, resolvedCWD, "cwd_not_directory", fmt.Sprintf("Working directory is not a directory: %s", resolvedCWD))
	}

	commandName, commandArgs := shellProgram(commandText)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, commandName, commandArgs...)
	cmd.Dir = resolvedCWD
	stdoutRaw, stdoutErr := cmd.Output()
	stderrText := ""
	if stdoutErr != nil {
		var exitErr *exec.ExitError
		if errors.As(stdoutErr, &exitErr) {
			stderrText = string(exitErr.Stderr)
		}
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return RunCommandResult{
			Success:  false,
			Command:  commandText,
			CWD:      resolvedCWD,
			ExitCode: TimeoutExitCode,
			Stdout:   string(stdoutRaw),
			Stderr:   stderrText,
			TimedOut: true,
			Timeout:  timeoutSeconds,
			Error: map[string]any{
				"code": "timed_out",
				"message": fmt.Sprintf(
					"Command exceeded the %ds timeout. Retry with a larger `timeout` argument, or set `run_in_background=true` to queue it as a task and poll with wait_task/get_task.",
					timeoutSeconds,
				),
			},
			Hint: "consider_run_command_stream",
		}
	}

	if stdoutErr == nil {
		return RunCommandResult{
			Success:  true,
			Command:  commandText,
			CWD:      resolvedCWD,
			ExitCode: 0,
			Stdout:   string(stdoutRaw),
			Stderr:   "",
			TimedOut: false,
		}
	}

	var exitErr *exec.ExitError
	if errors.As(stdoutErr, &exitErr) {
		return RunCommandResult{
			Success:  false,
			Command:  commandText,
			CWD:      resolvedCWD,
			ExitCode: exitErr.ExitCode(),
			Stdout:   string(stdoutRaw),
			Stderr:   string(exitErr.Stderr),
			TimedOut: false,
		}
	}

	return RunCommandResult{
		Success:  false,
		Command:  commandText,
		CWD:      resolvedCWD,
		ExitCode: TimeoutExitCode,
		Stdout:   string(stdoutRaw),
		Stderr:   stdoutErr.Error(),
		TimedOut: false,
		Error: map[string]any{
			"code":    "command_start_failed",
			"message": stdoutErr.Error(),
		},
	}
}

func taskPollingRoot() string {
	return filepath.Join(os.TempDir(), "notion-local-ops-mcp-go-task7")
}

func writeTaskResult(root string, task taskstore.Task, result process.Result, status, summary string) error {
	task.Status = status
	taskDir := filepath.Join(root, "tasks", task.ID)
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(taskDir, "meta.json"), payload, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(taskDir, "summary.txt"), []byte(summary), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(taskDir, "stdout.log"), []byte(result.Stdout), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(taskDir, "stderr.log"), []byte(result.Stderr), 0o600); err != nil {
		return err
	}
	return nil
}

func runStreamCommand(ctx context.Context, workspace, commandText, cwd string, timeoutSeconds int, onStart taskrunner.StartCallback) (process.Result, error) {
	command, err := shellCommand(workspace, commandText, cwd)
	if err != nil {
		return process.Result{
			ExitCode: TimeoutExitCode,
			Stderr:   err.Error(),
		}, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return process.Result{ExitCode: TimeoutExitCode, Stderr: err.Error()}, process.ErrCommandStart
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return process.Result{ExitCode: TimeoutExitCode, Stderr: err.Error()}, process.ErrCommandStart
	}
	if err := cmd.Start(); err != nil {
		return process.Result{ExitCode: TimeoutExitCode, Stderr: err.Error()}, process.ErrCommandStart
	}
	if cmd.Process != nil && onStart != nil {
		onStart(cmd.Process.Pid)
	}

	type pipeRead struct {
		data []byte
		err  error
	}

	stdoutCh := make(chan pipeRead, 1)
	stderrCh := make(chan pipeRead, 1)
	go func() {
		data, readErr := io.ReadAll(stdoutPipe)
		stdoutCh <- pipeRead{data: data, err: readErr}
	}()
	go func() {
		data, readErr := io.ReadAll(stderrPipe)
		stderrCh <- pipeRead{data: data, err: readErr}
	}()

	waitErr := cmd.Wait()
	stdoutRead := <-stdoutCh
	stderrRead := <-stderrCh

	result := process.Result{
		Stdout:   string(stdoutRead.data),
		Stderr:   string(stderrRead.data),
		ExitCode: 0,
	}

	if stdoutRead.err != nil || stderrRead.err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			result.ExitCode = TimeoutExitCode
			if result.Stderr == "" {
				result.Stderr = ctx.Err().Error()
			}
			return result, context.Canceled
		}
		if stdoutRead.err != nil {
			result.Stderr = stdoutRead.err.Error()
		} else {
			result.Stderr = stderrRead.err.Error()
		}
		result.ExitCode = TimeoutExitCode
		return result, process.ErrCommandStart
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.ExitCode = TimeoutExitCode
		if result.Stderr == "" {
			result.Stderr = ctx.Err().Error()
		}
		return result, ctx.Err()
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		result.ExitCode = TimeoutExitCode
		if result.Stderr == "" {
			result.Stderr = ctx.Err().Error()
		}
		return result, context.Canceled
	}
	if waitErr == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, waitErr
	}

	result.Stderr = waitErr.Error()
	result.ExitCode = TimeoutExitCode
	return result, process.ErrCommandStart
}

func shellCommand(workspace, commandText, cwd string) (process.Command, error) {
	workingDir, err := resolveCommandCWD(workspace, cwd)
	if err != nil {
		return process.Command{}, err
	}

	name, args := shellProgram(commandText)
	return process.Command{
		Name: name,
		Args: args,
		Dir:  workingDir,
	}, nil
}

func resolveCommandCWD(workspace, cwd string) (string, error) {
	workingDir := EffectiveCWD(workspace)
	if cwd == "" {
		if info, err := os.Stat(workingDir); err == nil && info.IsDir() {
			return workingDir, nil
		}
		return workspace, nil
	}

	resolved, err := fsx.ResolvePath(workspace, cwd)
	if err != nil {
		return resolved, err
	}
	info, statErr := os.Stat(resolved)
	if statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return resolved, nil
		}
		return resolved, statErr
	}
	if !info.IsDir() {
		return resolved, nil
	}
	return resolved, nil
}

func shellProgram(commandText string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", commandText}
	}
	return "sh", []string{"-lc", commandText}
}

func commandPathErrorResult(commandText, cwd, code, message string) RunCommandResult {
	return RunCommandResult{
		Success:  false,
		Command:  commandText,
		CWD:      cwd,
		ExitCode: TimeoutExitCode,
		Stdout:   "",
		Stderr:   "",
		TimedOut: false,
		Error: map[string]any{
			"code":    code,
			"message": message,
		},
	}
}
