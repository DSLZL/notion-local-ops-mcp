package fsx

import (
	"errors"
	"path/filepath"
	"strings"
)

var ErrPathEscapesWorkspace = errors.New("path escapes workspace")

func ResolvePath(workspace, input string) (string, error) {
	root := filepath.Clean(workspace)
	resolved := filepath.Clean(filepath.Join(root, input))
	if filepath.IsAbs(input) {
		resolved = filepath.Clean(input)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrPathEscapesWorkspace
	}
	return resolved, nil
}
