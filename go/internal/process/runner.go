package process

import (
	"errors"
	"io"
	"os/exec"
)

type Command struct {
	Name string
	Args []string
	Dir  string
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

var ErrCommandStart = errors.New("command start failed")

func Run(cmd Command) (Result, error) {
	command := exec.Command(cmd.Name, cmd.Args...)
	if cmd.Dir != "" {
		command.Dir = cmd.Dir
	}
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return Result{ExitCode: -1, Stderr: err.Error()}, ErrCommandStart
	}
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		return Result{ExitCode: -1, Stderr: err.Error()}, ErrCommandStart
	}
	if err := command.Start(); err != nil {
		return Result{ExitCode: -1, Stderr: err.Error()}, ErrCommandStart
	}

	stdoutRaw, stdoutErr := io.ReadAll(stdoutPipe)
	stderrRaw, stderrErr := io.ReadAll(stderrPipe)
	waitErr := command.Wait()

	result := Result{
		Stdout:   string(stdoutRaw),
		Stderr:   string(stderrRaw),
		ExitCode: 0,
	}
	if stdoutErr != nil || stderrErr != nil {
		if stdoutErr != nil {
			result.Stderr = stdoutErr.Error()
		} else {
			result.Stderr = stderrErr.Error()
		}
		result.ExitCode = -1
		return result, ErrCommandStart
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
	result.ExitCode = -1
	return result, ErrCommandStart
}
