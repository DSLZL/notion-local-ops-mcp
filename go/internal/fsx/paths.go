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

// ResolveWritePath resolves a file path for write operations. It first tries
// the workspace root; if that fails with ErrPathEscapesWorkspace, it checks
// each directory in extraDirs as an allowed prefix.
func ResolveWritePath(workspace, input string, extraDirs []string) (string, error) {
	resolved, err := ResolvePath(workspace, input)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, ErrPathEscapesWorkspace) {
		return "", err
	}

	// The path escapes the workspace. Check extra allowed directories.
	if !filepath.IsAbs(input) {
		return "", ErrPathEscapesWorkspace
	}
	cleaned := filepath.Clean(input)
	for _, dir := range extraDirs {
		allowed := filepath.Clean(dir)
		if allowed == "" {
			continue
		}
		rel, relErr := filepath.Rel(allowed, cleaned)
		if relErr != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		// Path is under this allowed directory.
		return cleaned, nil
	}

	return "", ErrPathEscapesWorkspace
}
