package tools

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GitStatusEntry struct {
	Path           string `json:"path"`
	IndexStatus    string `json:"index_status"`
	WorktreeStatus string `json:"worktree_status"`
}

type GitStatusResult struct {
	Success   bool             `json:"success"`
	CWD       string           `json:"cwd,omitempty"`
	RepoRoot  string           `json:"repo_root,omitempty"`
	Branch    string           `json:"branch,omitempty"`
	Clean     bool             `json:"clean,omitempty"`
	Staged    []string         `json:"staged,omitempty"`
	Unstaged  []string         `json:"unstaged,omitempty"`
	Untracked []string         `json:"untracked,omitempty"`
	Entries   []GitStatusEntry `json:"entries,omitempty"`
	Error     *GitErrorPayload `json:"error,omitempty"`
	Stderr    string           `json:"stderr,omitempty"`
}

type GitFileDiff struct {
	Path      string `json:"path"`
	Diff      string `json:"diff"`
	Truncated bool   `json:"truncated"`
	Bytes     int    `json:"bytes"`
	Added     *int   `json:"added,omitempty"`
	Removed   *int   `json:"removed,omitempty"`
	Binary    bool   `json:"binary"`
}

type GitDiffResult struct {
	Success    bool             `json:"success"`
	CWD        string           `json:"cwd,omitempty"`
	RepoRoot   string           `json:"repo_root,omitempty"`
	Staged     bool             `json:"staged,omitempty"`
	Files      []string         `json:"files,omitempty"`
	FileDiffs  []GitFileDiff    `json:"file_diffs,omitempty"`
	Diff       string           `json:"diff,omitempty"`
	Truncated  bool             `json:"truncated,omitempty"`
	TotalBytes int              `json:"total_bytes,omitempty"`
	Error      *GitErrorPayload `json:"error,omitempty"`
}

type GitLogEntry struct {
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	Summary     string `json:"summary"`
	Author      string `json:"author"`
	CommittedAt string `json:"committed_at"`
}

type GitLogResult struct {
	Success  bool             `json:"success"`
	CWD      string           `json:"cwd,omitempty"`
	RepoRoot string           `json:"repo_root,omitempty"`
	Branch   string           `json:"branch,omitempty"`
	Entries  []GitLogEntry    `json:"entries,omitempty"`
	Error    *GitErrorPayload `json:"error,omitempty"`
}

type GitShowResult struct {
	Success     bool             `json:"success"`
	CWD         string           `json:"cwd,omitempty"`
	RepoRoot    string           `json:"repo_root,omitempty"`
	Ref         string           `json:"ref,omitempty"`
	Commit      string           `json:"commit,omitempty"`
	ShortCommit string           `json:"short_commit,omitempty"`
	Summary     string           `json:"summary,omitempty"`
	Author      string           `json:"author,omitempty"`
	CommittedAt string           `json:"committed_at,omitempty"`
	Parents     []string         `json:"parents,omitempty"`
	Body        string           `json:"body,omitempty"`
	Files       []string         `json:"files,omitempty"`
	FileDiffs   []GitFileDiff    `json:"file_diffs,omitempty"`
	Diff        string           `json:"diff,omitempty"`
	Truncated   bool             `json:"truncated,omitempty"`
	TotalBytes  int              `json:"total_bytes,omitempty"`
	Error       *GitErrorPayload `json:"error,omitempty"`
}

type GitBlameEntry struct {
	Line        int    `json:"line"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	Author      string `json:"author"`
	AuthorTime  string `json:"author_time"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
}

type GitBlameResult struct {
	Success  bool             `json:"success"`
	CWD      string           `json:"cwd,omitempty"`
	RepoRoot string           `json:"repo_root,omitempty"`
	Path     string           `json:"path,omitempty"`
	Ref      string           `json:"ref,omitempty"`
	Entries  []GitBlameEntry  `json:"entries,omitempty"`
	Error    *GitErrorPayload `json:"error,omitempty"`
}

type GitCommitResult struct {
	Success     bool             `json:"success"`
	CWD         string           `json:"cwd,omitempty"`
	RepoRoot    string           `json:"repo_root,omitempty"`
	Branch      string           `json:"branch,omitempty"`
	Commit      string           `json:"commit,omitempty"`
	ShortCommit string           `json:"short_commit,omitempty"`
	Summary     string           `json:"summary,omitempty"`
	Files       []string         `json:"files,omitempty"`
	Amended     bool             `json:"amended,omitempty"`
	AllowEmpty  bool             `json:"allow_empty,omitempty"`
	DryRun      bool             `json:"dry_run,omitempty"`
	WouldStage  []string         `json:"would_stage,omitempty"`
	CommitArgs  []string         `json:"commit_args,omitempty"`
	Error       *GitErrorPayload `json:"error,omitempty"`
}

type gitRunResult struct {
	Stdout string
	Stderr string
	Code   int
}

func GitStatus(repoRoot string) GitStatusResult {
	cwd := repoRoot
	root, branch, failure := requireRepo(cwd)
	if failure != nil {
		return *failure
	}

	result := runGit(cwd, "status", "--short", "--branch")
	if result.Code != 0 {
		return GitStatusResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("git_status_failed", fallbackMessage(result.Stderr, "git status failed.")),
		}
	}

	var staged []string
	var unstaged []string
	var untracked []string
	var entries []GitStatusEntry

	for _, line := range strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "## ") {
			continue
		}
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		rawPath := line[3:]
		path := rawPath
		if parts := strings.SplitN(rawPath, " -> ", 2); len(parts) == 2 {
			path = parts[1]
		}
		entry := GitStatusEntry{
			Path:           path,
			IndexStatus:    string(code[0]),
			WorktreeStatus: string(code[1]),
		}
		entries = append(entries, entry)
		if code == "??" {
			untracked = append(untracked, path)
			continue
		}
		if code[0] != ' ' {
			staged = append(staged, path)
		}
		if code[1] != ' ' {
			unstaged = append(unstaged, path)
		}
	}

	return GitStatusResult{
		Success:   true,
		CWD:       cwd,
		RepoRoot:  root,
		Branch:    branch,
		Clean:     len(entries) == 0,
		Staged:    staged,
		Unstaged:  unstaged,
		Untracked: untracked,
		Entries:   entries,
	}
}

func GitDiff(repoRoot string, staged bool, paths []string, maxBytes, perFileMaxBytes int) GitDiffResult {
	cwd := repoRoot
	root, _, failure := requireRepo(cwd)
	if failure != nil {
		return GitDiffResult{
			Success: false,
			CWD:     cwd,
			Error:   failure.Error,
		}
	}

	normalizedPaths := normalizePathspecs(paths, cwd, root)
	args := []string{"diff", "--no-color"}
	if staged {
		args = append(args, "--cached")
	}
	if len(normalizedPaths) > 0 {
		args = append(args, "--")
		args = append(args, normalizedPaths...)
	}

	result := runGit(cwd, args...)
	if result.Code != 0 {
		return GitDiffResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("git_diff_failed", fallbackMessage(result.Stderr, "git diff failed.")),
		}
	}

	numstatArgs := []string{"diff", "--numstat"}
	if staged {
		numstatArgs = append(numstatArgs, "--cached")
	}
	if len(normalizedPaths) > 0 {
		numstatArgs = append(numstatArgs, "--")
		numstatArgs = append(numstatArgs, normalizedPaths...)
	}
	numstatResult := runGit(cwd, numstatArgs...)
	stats := parseNumstat(numstatResult.Stdout)

	fileDiffs := []GitFileDiff{}
	for _, chunk := range splitDiffByFile(result.Stdout) {
		diffBytes := []byte(chunk.Diff)
		truncated := len(diffBytes) > perFileMaxBytes
		rendered := chunk.Diff
		if truncated {
			rendered = string(diffBytes[:perFileMaxBytes])
		}
		stat := stats[chunk.Path]
		fileDiffs = append(fileDiffs, GitFileDiff{
			Path:      chunk.Path,
			Diff:      rendered,
			Truncated: truncated,
			Bytes:     len(diffBytes),
			Added:     stat.Added,
			Removed:   stat.Removed,
			Binary:    stat.Binary,
		})
	}

	fullBytes := []byte(result.Stdout)
	truncated := len(fullBytes) > maxBytes
	diffText := result.Stdout
	if truncated {
		diffText = string(fullBytes[:maxBytes])
	}

	files := make([]string, 0, len(fileDiffs))
	for _, item := range fileDiffs {
		files = append(files, item.Path)
	}

	return GitDiffResult{
		Success:    true,
		CWD:        cwd,
		RepoRoot:   root,
		Staged:     staged,
		Files:      files,
		FileDiffs:  fileDiffs,
		Diff:       diffText,
		Truncated:  truncated,
		TotalBytes: len(fullBytes),
	}
}

func GitLog(repoRoot string, limit int) GitLogResult {
	cwd := repoRoot
	root, branch, failure := requireRepo(cwd)
	if failure != nil {
		return GitLogResult{
			Success: false,
			CWD:     cwd,
			Error:   failure.Error,
		}
	}

	if limit < 1 {
		limit = 1
	}
	result := runGit(cwd, "log", "--max-count="+strconv.Itoa(limit), "--pretty=format:%H%x1f%h%x1f%s%x1f%an%x1f%aI")
	if result.Code != 0 {
		return GitLogResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("git_log_failed", fallbackMessage(result.Stderr, "git log failed.")),
		}
	}

	var entries []GitLogEntry
	for _, line := range strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) != 5 {
			continue
		}
		entries = append(entries, GitLogEntry{
			Commit:      parts[0],
			ShortCommit: parts[1],
			Summary:     parts[2],
			Author:      parts[3],
			CommittedAt: parts[4],
		})
	}

	return GitLogResult{
		Success:  true,
		CWD:      cwd,
		RepoRoot: root,
		Branch:   branch,
		Entries:  entries,
	}
}

func GitShow(repoRoot, ref string, maxBytes, perFileMaxBytes int) GitShowResult {
	cwd := repoRoot
	root, _, failure := requireRepo(cwd)
	if failure != nil {
		return GitShowResult{
			Success: false,
			CWD:     cwd,
			Error:   failure.Error,
		}
	}
	if ref == "" {
		ref = "HEAD"
	}

	meta := runGit(cwd, "show", "--no-color", "--no-patch", "--pretty=format:%H%x1f%h%x1f%s%x1f%an%x1f%aI%x1f%P%x1f%B", ref)
	if meta.Code != 0 {
		return GitShowResult{
			Success: false,
			CWD:     cwd,
			Ref:     ref,
			Error:   gitError("git_show_failed", fallbackMessage(meta.Stderr, "git show failed for ref "+ref)),
		}
	}

	parts := strings.SplitN(meta.Stdout, "\x1f", 7)
	if len(parts) < 7 {
		return GitShowResult{
			Success: false,
			CWD:     cwd,
			Ref:     ref,
			Error:   gitError("git_show_failed", "Unexpected git show output."),
		}
	}

	diffResult := runGit(cwd, "show", "--no-color", "--format=", ref)
	if diffResult.Code != 0 {
		return GitShowResult{
			Success: false,
			CWD:     cwd,
			Ref:     ref,
			Error:   gitError("git_show_failed", fallbackMessage(diffResult.Stderr, "git show diff failed for ref "+ref)),
		}
	}

	fileDiffs := []GitFileDiff{}
	files := []string{}
	for _, chunk := range splitDiffByFile(diffResult.Stdout) {
		diffBytes := []byte(chunk.Diff)
		truncated := len(diffBytes) > perFileMaxBytes
		rendered := chunk.Diff
		if truncated {
			rendered = string(diffBytes[:perFileMaxBytes])
		}
		files = append(files, chunk.Path)
		fileDiffs = append(fileDiffs, GitFileDiff{
			Path:      chunk.Path,
			Diff:      rendered,
			Truncated: truncated,
			Bytes:     len(diffBytes),
		})
	}

	fullBytes := []byte(diffResult.Stdout)
	truncated := len(fullBytes) > maxBytes
	diffText := diffResult.Stdout
	if truncated {
		diffText = string(fullBytes[:maxBytes])
	}

	body := strings.TrimRight(parts[6], "\n")
	parentText := strings.TrimSpace(parts[5])
	var parents []string
	if parentText != "" {
		parents = strings.Split(parentText, " ")
	}

	return GitShowResult{
		Success:     true,
		CWD:         cwd,
		RepoRoot:    root,
		Ref:         ref,
		Commit:      parts[0],
		ShortCommit: parts[1],
		Summary:     parts[2],
		Author:      parts[3],
		CommittedAt: parts[4],
		Parents:     parents,
		Body:        body,
		Files:       files,
		FileDiffs:   fileDiffs,
		Diff:        diffText,
		Truncated:   truncated,
		TotalBytes:  len(fullBytes),
	}
}

func GitBlame(repoRoot, path, ref string, startLine, endLine *int) GitBlameResult {
	cwd := repoRoot
	root, _, failure := requireRepo(cwd)
	if failure != nil {
		return GitBlameResult{
			Success: false,
			CWD:     cwd,
			Error:   failure.Error,
		}
	}

	// git_blame declares a repository-relative path contract, so resolve
	// relative inputs from the repo root rather than the session cwd.
	normalized := normalizePathspec(path, root, root)
	args := []string{"blame", "--porcelain"}
	if startLine != nil || endLine != nil {
		a := 1
		if startLine != nil && *startLine > 0 {
			a = *startLine
		}
		b := a
		if endLine != nil {
			b = *endLine
		}
		args = append(args, "-L", strconv.Itoa(a)+","+strconv.Itoa(b))
	}
	if ref != "" {
		args = append(args, ref)
	}
	args = append(args, "--", normalized)

	result := runGit(root, args...)
	if result.Code != 0 {
		return GitBlameResult{
			Success: false,
			CWD:     cwd,
			Path:    path,
			Ref:     ref,
			Error:   gitError("git_blame_failed", fallbackMessage(result.Stderr, "git blame failed.")),
		}
	}

	type blameMeta struct {
		SHA       string
		FinalLine int
		Fields    map[string]string
	}

	metaBySHA := map[string]map[string]string{}
	var entries []GitBlameEntry
	var current *blameMeta

	for _, raw := range strings.Split(strings.ReplaceAll(result.Stdout, "\r\n", "\n"), "\n") {
		if raw == "" {
			continue
		}
		if current == nil {
			parts := strings.Split(raw, " ")
			if len(parts) < 3 {
				continue
			}
			lineNo, err := strconv.Atoi(parts[2])
			if err != nil {
				continue
			}
			fields := metaBySHA[parts[0]]
			if fields == nil {
				fields = map[string]string{}
				metaBySHA[parts[0]] = fields
			}
			current = &blameMeta{
				SHA:       parts[0],
				FinalLine: lineNo,
				Fields:    fields,
			}
			continue
		}
		if strings.HasPrefix(raw, "\t") {
			info := metaBySHA[current.SHA]
			entries = append(entries, GitBlameEntry{
				Line:        current.FinalLine,
				Commit:      current.SHA,
				ShortCommit: shortSHA(current.SHA),
				Author:      info["author"],
				AuthorTime:  info["author-time"],
				Summary:     info["summary"],
				Content:     strings.TrimPrefix(raw, "\t"),
			})
			current = nil
			continue
		}
		key, value, ok := strings.Cut(raw, " ")
		if ok && isBlameMetaKey(key) {
			current.Fields[key] = value
		}
	}

	return GitBlameResult{
		Success:  true,
		CWD:      cwd,
		RepoRoot: root,
		Path:     normalized,
		Ref:      ref,
		Entries:  entries,
	}
}

func GitCommit(repoRoot, message string, paths []string, stageAll, amend, allowEmpty bool, author string, signOff, dryRun bool) GitCommitResult {
	cwd := repoRoot
	root, branch, failure := requireRepo(cwd)
	if failure != nil {
		return GitCommitResult{
			Success: false,
			CWD:     cwd,
			Error:   failure.Error,
		}
	}

	normalizedPaths := normalizePathspecs(paths, cwd, root)
	if stageAll {
		if !dryRun {
			stageResult := runGit(cwd, "add", "-A")
			if stageResult.Code != 0 {
				return GitCommitResult{
					Success: false,
					CWD:     cwd,
					Error:   gitError("git_add_failed", fallbackMessage(stageResult.Stderr, "git add -A failed.")),
				}
			}
		}
	} else if len(normalizedPaths) > 0 {
		if !dryRun {
			args := append([]string{"add", "--"}, normalizedPaths...)
			stageResult := runGit(cwd, args...)
			if stageResult.Code != 0 {
				return GitCommitResult{
					Success: false,
					CWD:     cwd,
					Error:   gitError("git_add_failed", fallbackMessage(stageResult.Stderr, "git add failed.")),
				}
			}
		}
	}

	stagedResult := runGit(cwd, "diff", "--cached", "--name-only")
	stagedFiles := nonEmptyLines(stagedResult.Stdout)
	var wouldStageFiles []string
	if stageAll {
		pending := runGit(cwd, "status", "--porcelain")
		for _, line := range nonEmptyLines(pending.Stdout) {
			if len(line) >= 4 {
				path := line[3:]
				if parts := strings.SplitN(path, " -> ", 2); len(parts) == 2 {
					path = parts[1]
				}
				wouldStageFiles = append(wouldStageFiles, path)
			}
		}
	} else if len(normalizedPaths) > 0 {
		wouldStageFiles = append(wouldStageFiles, normalizedPaths...)
	}

	effectiveFiles := append([]string{}, stagedFiles...)
	for _, item := range wouldStageFiles {
		if !containsString(effectiveFiles, item) {
			effectiveFiles = append(effectiveFiles, item)
		}
	}

	if len(effectiveFiles) == 0 && !allowEmpty && !amend {
		return GitCommitResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("nothing_to_commit", "No staged changes to commit."),
		}
	}

	commitArgs := []string{"commit", "-m", message}
	if amend {
		commitArgs = append(commitArgs, "--amend")
	}
	if allowEmpty {
		commitArgs = append(commitArgs, "--allow-empty")
	}
	if author != "" {
		commitArgs = append(commitArgs, "--author", author)
	}
	if signOff {
		commitArgs = append(commitArgs, "--signoff")
	}

	if dryRun {
		return GitCommitResult{
			Success:    true,
			CWD:        cwd,
			RepoRoot:   root,
			Branch:     branch,
			Summary:    message,
			Files:      effectiveFiles,
			Amended:    amend,
			AllowEmpty: allowEmpty,
			DryRun:     true,
			WouldStage: wouldStageFiles,
			CommitArgs: commitArgs,
		}
	}

	commitResult := runGit(cwd, commitArgs...)
	if commitResult.Code != 0 {
		return GitCommitResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("git_commit_failed", fallbackMessage(commitResult.Stderr, fallbackMessage(commitResult.Stdout, "git commit failed."))),
		}
	}

	headResult := runGit(cwd, "rev-parse", "HEAD")
	commitHash := strings.TrimSpace(headResult.Stdout)
	committedFiles := stagedFiles
	if amend {
		changed := runGit(cwd, "show", "--name-only", "--pretty=format:", commitHash)
		committedFiles = nonEmptyLines(changed.Stdout)
	}

	return GitCommitResult{
		Success:     true,
		CWD:         cwd,
		RepoRoot:    root,
		Branch:      branch,
		Commit:      commitHash,
		ShortCommit: shortSHA(commitHash),
		Summary:     message,
		Files:       committedFiles,
		Amended:     amend,
		AllowEmpty:  allowEmpty,
		DryRun:      false,
	}
}

type diffChunk struct {
	Path string
	Diff string
}

type numstatEntry struct {
	Added   *int
	Removed *int
	Binary  bool
}

func splitDiffByFile(diffText string) []diffChunk {
	if diffText == "" {
		return nil
	}

	lines := strings.SplitAfter(strings.ReplaceAll(diffText, "\r\n", "\n"), "\n")
	var chunks []diffChunk
	var buffer strings.Builder
	currentPath := ""

	flush := func() {
		if buffer.Len() == 0 {
			return
		}
		path := currentPath
		if path == "" {
			path = "(unknown)"
		}
		chunks = append(chunks, diffChunk{Path: path, Diff: buffer.String()})
		buffer.Reset()
	}

	for _, raw := range lines {
		if strings.HasPrefix(raw, "diff --git ") {
			flush()
			header := strings.TrimSuffix(strings.TrimPrefix(raw, "diff --git "), "\n")
			parts := strings.Split(header, " ")
			if len(parts) > 0 {
				candidate := parts[len(parts)-1]
				if strings.HasPrefix(candidate, "b/") {
					currentPath = candidate[2:]
				} else {
					currentPath = candidate
				}
			}
		} else if strings.HasPrefix(raw, "+++ ") && buffer.Len() > 0 {
			marker := strings.TrimSuffix(strings.TrimPrefix(raw, "+++ "), "\n")
			if strings.HasPrefix(marker, "b/") {
				currentPath = marker[2:]
			} else if marker != "/dev/null" {
				currentPath = marker
			}
		}
		buffer.WriteString(raw)
	}
	flush()
	return chunks
}

func parseNumstat(stdout string) map[string]numstatEntry {
	stats := map[string]numstatEntry{}
	for _, line := range strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}
		var added *int
		var removed *int
		binary := parts[0] == "-" && parts[1] == "-"
		if !binary {
			if value, err := strconv.Atoi(parts[0]); err == nil {
				added = &value
			}
			if value, err := strconv.Atoi(parts[1]); err == nil {
				removed = &value
			}
		}
		stats[parts[2]] = numstatEntry{
			Added:   added,
			Removed: removed,
			Binary:  binary,
		}
	}
	return stats
}

func requireRepo(cwd string) (string, string, *GitStatusResult) {
	info, err := os.Stat(cwd)
	if err != nil {
		return "", "", &GitStatusResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("cwd_not_found", "Working directory not found: "+cwd),
		}
	}
	if !info.IsDir() {
		return "", "", &GitStatusResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("cwd_not_directory", "Working directory is not a directory: "+cwd),
		}
	}

	rootResult := runGit(cwd, "rev-parse", "--show-toplevel")
	if rootResult.Code != 0 {
		return "", "", &GitStatusResult{
			Success: false,
			CWD:     cwd,
			Error:   gitError("not_a_git_repo", "Working directory is not inside a git repository."),
			Stderr:  strings.TrimSpace(rootResult.Stderr),
		}
	}

	branchResult := runGit(cwd, "branch", "--show-current")
	branch := strings.TrimSpace(branchResult.Stdout)
	if branch == "" {
		branch = "HEAD"
	}

	return strings.TrimSpace(rootResult.Stdout), branch, nil
}

func runGit(cwd string, args ...string) gitRunResult {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return gitRunResult{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String()), Code: 0}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return gitRunResult{Stdout: strings.TrimSpace(stdout.String()), Stderr: strings.TrimSpace(stderr.String()), Code: exitErr.ExitCode()}
	}
	return gitRunResult{Stdout: strings.TrimSpace(stdout.String()), Stderr: fallbackMessage(err.Error(), stderr.String()), Code: -1}
}

func normalizePathspecs(paths []string, cwd, repoRoot string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		result = append(result, normalizePathspec(path, cwd, repoRoot))
	}
	return result
}

func normalizePathspec(path, cwd, repoRoot string) string {
	raw := filepath.Clean(path)
	absolute := raw
	if !filepath.IsAbs(raw) {
		absolute = filepath.Join(cwd, raw)
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(repoRoot, absolute)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(path)
}

func gitError(code, message string) *GitErrorPayload {
	return &GitErrorPayload{Code: code, Message: message}
}

func fallbackMessage(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func nonEmptyLines(text string) []string {
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isBlameMetaKey(key string) bool {
	switch key {
	case "author", "author-time", "summary", "filename", "previous":
		return true
	default:
		return false
	}
}
