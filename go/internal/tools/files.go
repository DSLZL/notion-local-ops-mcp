package tools

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"notion-local-ops-mcp-go/internal/fsx"
)

type FileEntry struct {
	Name  string `json:"name,omitempty"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir,omitempty"`
}

type ListFilesOptions struct {
	Path            string
	Recursive       bool
	Limit           int
	Offset          int
	IncludeHidden   bool
	ExcludePatterns []string
}

type ListFilesResult struct {
	Success    bool        `json:"success"`
	BasePath   string      `json:"base_path"`
	Entries    []FileEntry `json:"entries"`
	Truncated  bool        `json:"truncated"`
	NextOffset *int        `json:"next_offset"`
}

type ReadTextOptions struct {
	Path               string
	Paths              []string
	StartLine          int
	LineLimit          int
	IncludeLineNumbers bool
}

type ReadTextResult struct {
	Success            bool   `json:"success"`
	Path               string `json:"path"`
	Content            string `json:"content"`
	Truncated          bool   `json:"truncated"`
	NextOffset         *int   `json:"next_offset"`
	OffsetUnit         string `json:"offset_unit"`
	StartLine          int    `json:"start_line"`
	EndLine            int    `json:"end_line"`
	IncludeLineNumbers bool   `json:"include_line_numbers"`
}

type BatchReadTextResult struct {
	Success bool             `json:"success"`
	Results []ReadTextResult `json:"results"`
}

type WriteFileResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	DryRun  bool   `json:"dry_run"`
	Bytes   int    `json:"bytes"`
	Summary string `json:"summary"`
}

type WriteFileOptions struct {
	Path          string  `json:"path"`
	Content       *string `json:"content,omitempty"`
	ContentBase64 *string `json:"content_base64,omitempty"`
	DryRun        bool    `json:"dry_run,omitempty"`
}

func ListFiles(workspace string, options any) (any, error) {
	switch input := options.(type) {
	case string:
		result, err := listFilesWithOptions(workspace, ListFilesOptions{Path: input})
		if err != nil {
			return nil, err
		}
		return flattenFileEntries(result.Entries), nil
	case ListFilesOptions:
		return listFilesWithOptions(workspace, input)
	default:
		return nil, fmt.Errorf("unsupported list_files options")
	}
}

func listFilesWithOptions(workspace string, options ListFilesOptions) (ListFilesResult, error) {
	path := options.Path
	if path == "" {
		path = "."
	}
	root, err := fsx.ResolvePath(workspace, path)
	if err != nil {
		return ListFilesResult{}, err
	}

	info, err := os.Stat(root)
	if err != nil {
		return ListFilesResult{}, err
	}
	if !info.IsDir() {
		return ListFilesResult{}, fmt.Errorf("path is not a directory: %s", root)
	}

	entries, err := collectVisibleEntries(root, options)
	if err != nil {
		return ListFilesResult{}, err
	}

	start := maxInt(options.Offset, 0)
	limit := options.Limit
	if limit <= 0 {
		limit = len(entries)
	}
	selected := entries
	if start < len(entries) {
		selected = entries[start:]
	} else {
		selected = []FileEntry{}
	}
	truncated := false
	var nextOffset *int
	if len(selected) > limit {
		selected = selected[:limit]
		truncated = true
		value := start + len(selected)
		nextOffset = &value
	}

	return ListFilesResult{
		Success:    true,
		BasePath:   root,
		Entries:    selected,
		Truncated:  truncated,
		NextOffset: nextOffset,
	}, nil
}

func collectVisibleEntries(root string, options ListFilesOptions) ([]FileEntry, error) {
	if !options.Recursive {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}

		visible := make([]FileEntry, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if !options.IncludeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			if matchesExcludePattern(name, name, options.ExcludePatterns) {
				continue
			}
			visible = append(visible, FileEntry{
				Name:  name,
				Path:  name,
				IsDir: entry.IsDir(),
			})
		}
		sort.Slice(visible, func(i, j int) bool {
			return visible[i].Path < visible[j].Path
		})
		return visible, nil
	}

	collected := make([]FileEntry, 0, 16)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		name := d.Name()
		if !options.IncludeHidden && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if matchesExcludePattern(name, relative, options.ExcludePatterns) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		collected = append(collected, FileEntry{
			Name:  name,
			Path:  relative,
			IsDir: d.IsDir(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Path < collected[j].Path
	})
	return collected, nil
}

func matchesExcludePattern(name, relative string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, relative); matched {
			return true
		}
	}
	return false
}

func flattenFileEntries(entries []FileEntry) []FileEntry {
	flat := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		flat = append(flat, FileEntry{Path: entry.Path})
	}
	return flat
}

func ReadText(workspace string, options any) (any, error) {
	switch input := options.(type) {
	case string:
		path, err := fsx.ResolvePath(workspace, input)
		if err != nil {
			return "", err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	case ReadTextOptions:
		if len(input.Paths) > 0 {
			results := make([]ReadTextResult, 0, len(input.Paths))
			for _, item := range input.Paths {
				result, err := readSingleText(workspace, ReadTextOptions{
					Path:               item,
					StartLine:          input.StartLine,
					LineLimit:          input.LineLimit,
					IncludeLineNumbers: input.IncludeLineNumbers,
				})
				if err != nil {
					return nil, err
				}
				results = append(results, result)
			}
			return BatchReadTextResult{Success: true, Results: results}, nil
		}
		return readSingleText(workspace, input)
	default:
		return nil, fmt.Errorf("unsupported read_text options")
	}
}

func readSingleText(workspace string, options ReadTextOptions) (ReadTextResult, error) {
	path, err := fsx.ResolvePath(workspace, options.Path)
	if err != nil {
		return ReadTextResult{}, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return ReadTextResult{}, err
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	startLine := options.StartLine
	if startLine <= 0 {
		startLine = 1
	}
	lineLimit := options.LineLimit
	if lineLimit <= 0 {
		lineLimit = 200
	}

	startIndex := startLine - 1
	if startIndex > len(lines) {
		startIndex = len(lines)
	}
	endIndex := startIndex + lineLimit
	if endIndex > len(lines) {
		endIndex = len(lines)
	}
	selected := lines[startIndex:endIndex]
	rendered := selected
	if options.IncludeLineNumbers {
		rendered = make([]string, 0, len(selected))
		for idx, line := range selected {
			rendered = append(rendered, fmt.Sprintf("%d: %s", startLine+idx, line))
		}
	}

	truncated := endIndex < len(lines)
	var nextOffset *int
	if truncated {
		value := endIndex + 1
		nextOffset = &value
	}
	endLine := startLine - 1
	if len(selected) > 0 {
		endLine = startLine + len(selected) - 1
	}

	return ReadTextResult{
		Success:            true,
		Path:               path,
		Content:            strings.Join(rendered, "\n"),
		Truncated:          truncated,
		NextOffset:         nextOffset,
		OffsetUnit:         "lines",
		StartLine:          startLine,
		EndLine:            endLine,
		IncludeLineNumbers: options.IncludeLineNumbers,
	}, nil
}

func WriteFile(workspace, input, content string, dryRun bool, extraWriteDirs []string) (WriteFileResult, error) {
	contentCopy := content
	return WriteFileWithOptions(workspace, WriteFileOptions{
		Path:    input,
		Content: &contentCopy,
		DryRun:  dryRun,
	}, extraWriteDirs)
}

func WriteFileWithOptions(workspace string, options WriteFileOptions, extraWriteDirs []string) (WriteFileResult, error) {
	data, err := resolveWriteFileContent(options)
	if err != nil {
		return WriteFileResult{}, err
	}

	path, err := fsx.ResolveWritePath(workspace, options.Path, extraWriteDirs)
	if err != nil {
		return WriteFileResult{}, err
	}

	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return WriteFileResult{}, fmt.Errorf("path is a directory: %s", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return WriteFileResult{}, err
	}

	if options.DryRun {
		return WriteFileResult{
			Success: true,
			Path:    path,
			DryRun:  true,
			Bytes:   len(data),
			Summary: "dry run: file write validated",
		}, nil
	}

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return WriteFileResult{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return WriteFileResult{}, err
	}
	return WriteFileResult{
		Success: true,
		Path:    path,
		DryRun:  false,
		Bytes:   len(data),
		Summary: "file written",
	}, nil
}

func resolveWriteFileContent(options WriteFileOptions) ([]byte, error) {
	hasContent := options.Content != nil
	hasContentBase64 := options.ContentBase64 != nil
	if hasContent == hasContentBase64 {
		return nil, fmt.Errorf("exactly one of content or content_base64 must be provided")
	}
	if hasContent {
		return []byte(*options.Content), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(*options.ContentBase64)
	if err != nil {
		return nil, fmt.Errorf("decode content_base64: %w", err)
	}
	return decoded, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
