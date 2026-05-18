package tools

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"notion-local-ops-mcp-go/internal/fsx"
)

type SearchOptions struct {
	Mode       string
	Path       string
	Pattern    string
	Query      string
	Before     int
	After      int
	Limit      int
	OutputMode string
	IgnoreCase bool
}

type SearchMatch struct {
	Path          string   `json:"path"`
	LineNumber    int      `json:"line_number,omitempty"`
	Line          string   `json:"line,omitempty"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

type GlobMatch struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type SearchCount struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

type SearchResult struct {
	Success     bool          `json:"success"`
	Mode        string        `json:"mode"`
	OutputMode  string        `json:"output_mode,omitempty"`
	Matches     []SearchMatch `json:"matches,omitempty"`
	GlobMatches []GlobMatch   `json:"glob_matches,omitempty"`
	Files       []string      `json:"files,omitempty"`
	Counts      []SearchCount `json:"counts,omitempty"`
	Truncated   bool          `json:"truncated"`
}

func Search(workspace string, arg1 any, arg2 ...string) (any, error) {
	switch input := arg1.(type) {
	case string:
		path := "."
		if len(arg2) > 0 {
			path = arg2[0]
		}
		return searchLegacy(workspace, input, path)
	case SearchOptions:
		return searchWithOptions(workspace, input)
	default:
		return nil, nil
	}
}

func searchLegacy(workspace, query, inputPath string) ([]string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	rootInput := inputPath
	if rootInput == "" {
		rootInput = "."
	}
	root, err := fsx.ResolvePath(workspace, rootInput)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return searchFileLiteral(workspace, root, query)
	}

	matches := make([]string, 0, 16)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() && path != root {
				return filepath.SkipDir
			}
			if path != root {
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}

		fileMatches, err := searchFileLiteral(workspace, path, query)
		if err != nil {
			return nil
		}
		matches = append(matches, fileMatches...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func searchWithOptions(workspace string, options SearchOptions) (SearchResult, error) {
	mode := options.Mode
	if mode == "" {
		mode = "regex"
	}
	searchPath := options.Path
	if searchPath == "" {
		searchPath = "."
	}
	root, err := fsx.ResolvePath(workspace, searchPath)
	if err != nil {
		return SearchResult{}, err
	}

	switch mode {
	case "glob":
		pattern := options.Pattern
		if pattern == "" {
			pattern = "*"
		}
		matches, err := globMatches(root, pattern)
		if err != nil {
			return SearchResult{}, err
		}
		return SearchResult{
			Success:     true,
			Mode:        mode,
			GlobMatches: limitGlobMatches(matches, options.Limit),
			Truncated:   options.Limit > 0 && len(matches) > options.Limit,
		}, nil
	case "text":
		return regexMatches(workspace, root, regexp.QuoteMeta(options.Query), mode, options)
	case "regex":
		return regexMatches(workspace, root, options.Pattern, mode, options)
	default:
		return SearchResult{Success: false, Mode: mode}, nil
	}
}

func regexMatches(workspace, root, pattern, mode string, options SearchOptions) (SearchResult, error) {
	if options.IgnoreCase {
		pattern = "(?i)" + pattern
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return SearchResult{}, err
	}

	files, err := searchCandidateFiles(root)
	if err != nil {
		return SearchResult{}, err
	}

	outputMode := options.OutputMode
	if outputMode == "" {
		outputMode = "content"
	}

	if outputMode == "files_with_matches" {
		matchedFiles := make([]string, 0, len(files))
		for _, path := range files {
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if compiled.Match(raw) {
				relative, relErr := filepath.Rel(workspace, path)
				if relErr != nil {
					relative = path
				}
				matchedFiles = append(matchedFiles, filepath.ToSlash(relative))
			}
		}
		truncated := false
		if options.Limit > 0 && len(matchedFiles) > options.Limit {
			matchedFiles = matchedFiles[:options.Limit]
			truncated = true
		}
		return SearchResult{
			Success:    true,
			Mode:       mode,
			OutputMode: outputMode,
			Files:      matchedFiles,
			Truncated:  truncated,
		}, nil
	}

	if outputMode == "count" {
		counts := make([]SearchCount, 0, len(files))
		for _, path := range files {
			raw, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			matches := compiled.FindAll(raw, -1)
			if len(matches) == 0 {
				continue
			}
			relative, relErr := filepath.Rel(workspace, path)
			if relErr != nil {
				relative = path
			}
			counts = append(counts, SearchCount{
				Path:  filepath.ToSlash(relative),
				Count: len(matches),
			})
		}
		truncated := false
		if options.Limit > 0 && len(counts) > options.Limit {
			counts = counts[:options.Limit]
			truncated = true
		}
		return SearchResult{
			Success:    true,
			Mode:       mode,
			OutputMode: outputMode,
			Counts:     counts,
			Truncated:  truncated,
		}, nil
	}

	matches := make([]SearchMatch, 0, 8)
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for idx, line := range lines {
			if !compiled.MatchString(line) {
				continue
			}
			relative, relErr := filepath.Rel(workspace, path)
			if relErr != nil {
				relative = path
			}
			match := SearchMatch{
				Path:       filepath.ToSlash(relative),
				LineNumber: idx + 1,
				Line:       line,
			}
			if options.Before > 0 {
				start := idx - options.Before
				if start < 0 {
					start = 0
				}
				match.ContextBefore = append([]string(nil), lines[start:idx]...)
			}
			if options.After > 0 {
				end := idx + 1 + options.After
				if end > len(lines) {
					end = len(lines)
				}
				match.ContextAfter = append([]string(nil), lines[idx+1:end]...)
			}
			matches = append(matches, match)
		}
	}

	truncated := false
	if options.Limit > 0 && len(matches) > options.Limit {
		matches = matches[:options.Limit]
		truncated = true
	}
	return SearchResult{
		Success:    true,
		Mode:       mode,
		OutputMode: outputMode,
		Matches:    matches,
		Truncated:  truncated,
	}, nil
}

func globMatches(root string, pattern string) ([]GlobMatch, error) {
	collected := make([]GlobMatch, 0, 8)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
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
		matched, err := filepath.Match(pattern, filepath.Base(relative))
		if err != nil {
			return err
		}
		if matched {
			collected = append(collected, GlobMatch{Path: relative, IsDir: d.IsDir()})
		}
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

func limitGlobMatches(matches []GlobMatch, limit int) []GlobMatch {
	if limit <= 0 || len(matches) <= limit {
		return matches
	}
	return matches[:limit]
}

func searchCandidateFiles(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{root}, nil
	}
	files := make([]string, 0, 8)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if strings.HasPrefix(d.Name(), ".") && path != root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func SearchForTest(query string) []string {
	matches, _ := searchLegacy(".", query, ".")
	return matches
}

func SearchLiteral(query string, corpus []string) []string {
	if query == "" {
		return nil
	}

	matches := make([]string, 0, len(corpus))
	for _, line := range corpus {
		if strings.Contains(line, query) {
			matches = append(matches, line)
		}
	}
	return matches
}

func searchFileLiteral(workspace, path, query string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	relative := path
	if rel, relErr := filepath.Rel(workspace, path); relErr == nil {
		relative = rel
	}

	lines := strings.Split(string(raw), "\n")
	matches := make([]string, 0, 4)
	for idx, line := range lines {
		if strings.Contains(line, query) {
			matches = append(matches, filepath.ToSlash(relative)+":"+strconv.Itoa(idx+1)+": "+line)
		}
	}
	return matches, nil
}
