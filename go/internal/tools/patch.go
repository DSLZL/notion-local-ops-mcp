package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"notion-local-ops-mcp-go/internal/fsx"
)

type ApplyPatchResult struct {
	Success            bool                 `json:"success"`
	Changes            []PatchChangeSummary `json:"changes,omitempty"`
	Files              []PatchFileSummary   `json:"files,omitempty"`
	Warnings           []string             `json:"warnings,omitempty"`
	Applied            bool                 `json:"applied"`
	Validated          bool                 `json:"validated"`
	Diff               string               `json:"diff,omitempty"`
	Error              *PatchErrorPayload   `json:"error,omitempty"`
	HunkIndex          int                  `json:"hunk_index,omitempty"`
	PatchLine          int                  `json:"patch_line,omitempty"`
	Expected           []string             `json:"expected,omitempty"`
	Candidates         []PatchCandidate     `json:"candidates,omitempty"`
	MatchCount         int                  `json:"match_count,omitempty"`
	ExpectedMatchCount int                  `json:"expected_match_count,omitempty"`
	MatchingLines      []int                `json:"matching_lines,omitempty"`
}

type PatchChangeSummary struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	FromPath string `json:"from_path,omitempty"`
	ToPath   string `json:"to_path,omitempty"`
}

type PatchFileSummary struct {
	Path         string   `json:"path"`
	Kind         string   `json:"kind"`
	LinesAdded   int      `json:"lines_added"`
	LinesRemoved int      `json:"lines_removed"`
	BytesBefore  int      `json:"bytes_before"`
	BytesAfter   int      `json:"bytes_after"`
	HunksApplied int      `json:"hunks_applied"`
	SHA256After  string   `json:"sha256_after,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type PatchErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PatchCandidate struct {
	Line       int     `json:"line"`
	Similarity float64 `json:"similarity"`
	Snippet    string  `json:"snippet"`
}

type patchOperation struct {
	Kind    string
	Path    string
	MoveTo  string
	AddLines []string
	Hunks   []patchHunk
}

type patchHunk struct {
	PatchLine int
	Lines     []patchLine
}

type patchLine struct {
	Kind byte
	Text string
}

type patchFailure struct {
	code              string
	message           string
	hunkIndex         int
	patchLine         int
	expected          []string
	candidates        []PatchCandidate
	matchCount        int
	expectedMatchCount int
	matchingLines     []int
}

func ApplyPatch(workspaceRoot, patch string, dryRun, validateOnly, returnDiff bool) ApplyPatchResult {
	ops, err := parsePatchDocument(patch)
	if err != nil {
		return failureResult(*err)
	}

	var (
		changes  []PatchChangeSummary
		files    []PatchFileSummary
		warnings []string
		diffs    []string
	)

	for _, op := range ops {
		change, fileSummary, diffText, fail := applyPatchOperation(workspaceRoot, op, dryRun || validateOnly)
		if fail != nil {
			return failureResult(*fail)
		}
		changes = append(changes, change)
		files = append(files, fileSummary)
		warnings = append(warnings, fileSummary.Warnings...)
		if returnDiff && diffText != "" {
			diffs = append(diffs, diffText)
		}
	}

	return ApplyPatchResult{
		Success:   true,
		Changes:   changes,
		Files:     files,
		Warnings:  warnings,
		Applied:   !dryRun && !validateOnly,
		Validated: dryRun || validateOnly,
		Diff:      strings.Join(diffs, ""),
	}
}

func parsePatchDocument(patch string) ([]patchOperation, *patchFailure) {
	lines := splitPatchLines(patch)
	if len(lines) == 0 || lines[0] != "*** Begin Patch" {
		return nil, &patchFailure{code: "invalid_patch", message: "Patch must start with '*** Begin Patch'."}
	}

	var ops []patchOperation
	for index := 1; index < len(lines); {
		line := lines[index]
		if line == "*** End Patch" {
			return ops, nil
		}

		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			op, next, err := parseAddFile(lines, index)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			index = next
		case strings.HasPrefix(line, "*** Delete File: "):
			ops = append(ops, patchOperation{Kind: "delete", Path: strings.TrimPrefix(line, "*** Delete File: ")})
			index++
		case strings.HasPrefix(line, "*** Update File: "):
			op, next, err := parseUpdateFile(lines, index)
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
			index = next
		default:
			return nil, &patchFailure{code: "invalid_patch", message: fmt.Sprintf("Unexpected patch header: %s", line)}
		}
	}

	return nil, &patchFailure{code: "invalid_patch", message: "Patch must end with '*** End Patch'."}
}

func parseAddFile(lines []string, start int) (patchOperation, int, *patchFailure) {
	op := patchOperation{Kind: "add", Path: strings.TrimPrefix(lines[start], "*** Add File: ")}
	index := start + 1
	for index < len(lines) && !isOperationHeader(lines[index]) {
		line := lines[index]
		if !strings.HasPrefix(line, "+") {
			return patchOperation{}, 0, &patchFailure{code: "invalid_patch", message: "Add file lines must start with '+'."}
		}
		op.AddLines = append(op.AddLines, strings.TrimPrefix(line, "+"))
		index++
	}
	return op, index, nil
}

func parseUpdateFile(lines []string, start int) (patchOperation, int, *patchFailure) {
	op := patchOperation{Kind: "update", Path: strings.TrimPrefix(lines[start], "*** Update File: ")}
	index := start + 1
	if index < len(lines) && strings.HasPrefix(lines[index], "*** Move to: ") {
		op.MoveTo = strings.TrimPrefix(lines[index], "*** Move to: ")
		index++
	}

	for index < len(lines) && !isOperationHeader(lines[index]) {
		if !strings.HasPrefix(lines[index], "@@") {
			return patchOperation{}, 0, &patchFailure{code: "invalid_patch", message: fmt.Sprintf("Unexpected hunk header: %s", lines[index])}
		}
		hunk, next, err := parseHunk(lines, index)
		if err != nil {
			return patchOperation{}, 0, err
		}
		op.Hunks = append(op.Hunks, hunk)
		index = next
	}

	if len(op.Hunks) == 0 && op.MoveTo == "" {
		return patchOperation{}, 0, &patchFailure{code: "invalid_patch", message: fmt.Sprintf("Update file patch has no changes: %s", op.Path)}
	}
	return op, index, nil
}

func parseHunk(lines []string, start int) (patchHunk, int, *patchFailure) {
	hunk := patchHunk{PatchLine: start + 1}
	index := start + 1
	hasChange := false

	for index < len(lines) && !isOperationHeader(lines[index]) && !strings.HasPrefix(lines[index], "@@") {
		line := lines[index]
		if line == "*** End of File" {
			index++
			continue
		}
		if len(line) == 0 {
			return patchHunk{}, 0, &patchFailure{code: "invalid_patch", message: "Hunk lines must start with ' ', '+' or '-'."}
		}
		kind := line[0]
		if kind != ' ' && kind != '+' && kind != '-' {
			return patchHunk{}, 0, &patchFailure{code: "invalid_patch", message: "Hunk lines must start with ' ', '+' or '-'."}
		}
		if kind == '+' || kind == '-' {
			hasChange = true
		}
		hunk.Lines = append(hunk.Lines, patchLine{Kind: kind, Text: line[1:]})
		index++
	}

	if !hasChange {
		return patchHunk{}, 0, &patchFailure{
			code:      "empty_hunk",
			message:   "Each @@ hunk must contain at least one '+' or '-' line.",
			patchLine: hunk.PatchLine,
		}
	}
	return hunk, index, nil
}

func isOperationHeader(line string) bool {
	return line == "*** End Patch" ||
		strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Delete File: ") ||
		strings.HasPrefix(line, "*** Update File: ")
}

func applyPatchOperation(workspaceRoot string, op patchOperation, validateOnly bool) (PatchChangeSummary, PatchFileSummary, string, *patchFailure) {
	switch op.Kind {
	case "add":
		return applyAddFile(workspaceRoot, op, validateOnly)
	case "delete":
		return applyDeleteFile(workspaceRoot, op, validateOnly)
	default:
		return applyUpdateFile(workspaceRoot, op, validateOnly)
	}
}

func applyAddFile(workspaceRoot string, op patchOperation, validateOnly bool) (PatchChangeSummary, PatchFileSummary, string, *patchFailure) {
	targetPath, err := fsx.ResolvePath(workspaceRoot, op.Path)
	if err != nil {
		return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "invalid_path", message: err.Error()}
	}
	after := joinContent(op.AddLines)
	if !validateOnly {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "write_failed", message: err.Error()}
		}
		if err := os.WriteFile(targetPath, []byte(after), 0o644); err != nil {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "write_failed", message: err.Error()}
		}
	}
	return PatchChangeSummary{Kind: "add", Path: targetPath}, PatchFileSummary{
		Path:         targetPath,
		Kind:         "add",
		LinesAdded:   len(op.AddLines),
		LinesRemoved: 0,
		BytesBefore:  0,
		BytesAfter:   len([]byte(after)),
		HunksApplied: 0,
		SHA256After:  sha256Hex(after),
	}, renderDiff(targetPath, "", after), nil
}

func applyDeleteFile(workspaceRoot string, op patchOperation, validateOnly bool) (PatchChangeSummary, PatchFileSummary, string, *patchFailure) {
	targetPath, err := fsx.ResolvePath(workspaceRoot, op.Path)
	if err != nil {
		return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "invalid_path", message: err.Error()}
	}
	beforeBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "file_not_found", message: err.Error()}
	}
	if !validateOnly {
		if err := os.Remove(targetPath); err != nil {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "delete_failed", message: err.Error()}
		}
	}
	beforeLines := splitContentLines(string(beforeBytes))
	return PatchChangeSummary{Kind: "delete", Path: targetPath}, PatchFileSummary{
		Path:         targetPath,
		Kind:         "delete",
		LinesAdded:   0,
		LinesRemoved: len(beforeLines),
		BytesBefore:  len(beforeBytes),
		BytesAfter:   0,
		HunksApplied: 0,
	}, renderDiff(targetPath, string(beforeBytes), ""), nil
}

func applyUpdateFile(workspaceRoot string, op patchOperation, validateOnly bool) (PatchChangeSummary, PatchFileSummary, string, *patchFailure) {
	sourcePath, err := fsx.ResolvePath(workspaceRoot, op.Path)
	if err != nil {
		return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "invalid_path", message: err.Error()}
	}
	targetPath := sourcePath
	if op.MoveTo != "" {
		targetPath, err = fsx.ResolvePath(workspaceRoot, op.MoveTo)
		if err != nil {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "invalid_path", message: err.Error()}
		}
	}

	beforeBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "file_not_found", message: err.Error()}
	}
	currentLines := splitContentLines(string(beforeBytes))
	linesAdded := 0
	linesRemoved := 0

	for hunkIndex, hunk := range op.Hunks {
		oldLines, newLines := hunkSequences(hunk)
		matches := findSequenceMatches(currentLines, oldLines)
		if len(matches) == 0 {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{
				code:       "patch_context_not_found",
				message:    fmt.Sprintf("Patch context not found for %s", op.Path),
				hunkIndex:  hunkIndex,
				expected:   oldLines,
				candidates: patchCandidates(currentLines, oldLines),
			}
		}
		if len(matches) > 1 {
			matchingLines := make([]int, 0, len(matches))
			for _, match := range matches {
				matchingLines = append(matchingLines, match+1)
			}
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{
				code:               "ambiguous_context_match",
				message:            fmt.Sprintf("Patch context matched %d locations for %s", len(matches), op.Path),
				matchCount:         len(matches),
				expectedMatchCount: 1,
				matchingLines:      matchingLines,
			}
		}

		start := matches[0]
		end := start + len(oldLines)
		replacement := make([]string, 0, len(currentLines)-len(oldLines)+len(newLines))
		replacement = append(replacement, currentLines[:start]...)
		replacement = append(replacement, newLines...)
		replacement = append(replacement, currentLines[end:]...)
		currentLines = replacement

		for _, line := range hunk.Lines {
			if line.Kind == '+' {
				linesAdded++
			}
			if line.Kind == '-' {
				linesRemoved++
			}
		}
	}

	after := joinContent(currentLines)
	warnings := []string{}
	if linesAdded > 0 && linesRemoved == 0 && len(op.Hunks) > 0 {
		warnings = append(warnings, "update inserted lines without removing any existing lines; verify this was intended")
	}

	if !validateOnly {
		if targetPath != sourcePath {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "write_failed", message: err.Error()}
			}
		}
		if err := os.WriteFile(targetPath, []byte(after), 0o644); err != nil {
			return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "write_failed", message: err.Error()}
		}
		if targetPath != sourcePath {
			if err := os.Remove(sourcePath); err != nil {
				return PatchChangeSummary{}, PatchFileSummary{}, "", &patchFailure{code: "delete_failed", message: err.Error()}
			}
		}
	}

	kind := "update"
	change := PatchChangeSummary{Kind: kind, Path: targetPath}
	if targetPath != sourcePath {
		kind = "move"
		change = PatchChangeSummary{Kind: "move", FromPath: sourcePath, ToPath: targetPath, Path: targetPath}
	}

	return change, PatchFileSummary{
		Path:         targetPath,
		Kind:         kind,
		LinesAdded:   linesAdded,
		LinesRemoved: linesRemoved,
		BytesBefore:  len(beforeBytes),
		BytesAfter:   len([]byte(after)),
		HunksApplied: len(op.Hunks),
		SHA256After:  sha256Hex(after),
		Warnings:     warnings,
	}, renderDiff(targetPath, string(beforeBytes), after), nil
}

func splitPatchLines(patch string) []string {
	normalized := strings.ReplaceAll(patch, "\r\n", "\n")
	normalized = strings.TrimRight(normalized, "\n")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "\n")
}

func splitContentLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	if normalized == "" {
		return []string{}
	}
	return strings.Split(normalized, "\n")
}

func joinContent(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func hunkSequences(hunk patchHunk) ([]string, []string) {
	var oldLines []string
	var newLines []string
	for _, line := range hunk.Lines {
		if line.Kind == ' ' || line.Kind == '-' {
			oldLines = append(oldLines, line.Text)
		}
		if line.Kind == ' ' || line.Kind == '+' {
			newLines = append(newLines, line.Text)
		}
	}
	return oldLines, newLines
}

func findSequenceMatches(lines, needle []string) []int {
	if len(needle) == 0 {
		return []int{0}
	}
	if len(lines) < len(needle) {
		return nil
	}
	var matches []int
	for start := 0; start <= len(lines)-len(needle); start++ {
		ok := true
		for offset := range needle {
			if lines[start+offset] != needle[offset] {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, start)
		}
	}
	return matches
}

func patchCandidates(lines, expected []string) []PatchCandidate {
	window := len(expected)
	if window == 0 {
		window = 1
	}
	if len(lines) == 0 {
		return nil
	}
	if len(lines) < window {
		window = len(lines)
	}

	candidates := make([]PatchCandidate, 0, len(lines))
	for start := 0; start <= len(lines)-window; start++ {
		snippetLines := lines[start : start+window]
		score := similarityScore(snippetLines, expected)
		candidates = append(candidates, PatchCandidate{
			Line:       start + 1,
			Similarity: score,
			Snippet:    strings.Join(snippetLines, "\n"),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Similarity == candidates[j].Similarity {
			return candidates[i].Line < candidates[j].Line
		}
		return candidates[i].Similarity > candidates[j].Similarity
	})
	if len(candidates) > 3 {
		return candidates[:3]
	}
	return candidates
}

func similarityScore(window, expected []string) float64 {
	compareLen := len(window)
	if len(expected) < compareLen {
		compareLen = len(expected)
	}
	if compareLen == 0 {
		return 0
	}
	matches := 0
	for i := 0; i < compareLen; i++ {
		if window[i] == expected[i] {
			matches++
		}
	}
	return float64(matches) / float64(compareLen)
}

func renderDiff(path, before, after string) string {
	var builder strings.Builder
	builder.WriteString("--- ")
	builder.WriteString(path)
	builder.WriteString("\n+++ ")
	builder.WriteString(path)
	builder.WriteString("\n@@\n")
	writeDiffBlock(&builder, '-', before)
	writeDiffBlock(&builder, '+', after)
	return builder.String()
}

func writeDiffBlock(builder *strings.Builder, prefix byte, content string) {
	lines := splitContentLines(content)
	if len(lines) == 0 {
		return
	}
	for _, line := range lines {
		builder.WriteByte(prefix)
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
}

func sha256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func failureResult(fail patchFailure) ApplyPatchResult {
	return ApplyPatchResult{
		Success: false,
		Error: &PatchErrorPayload{
			Code:    fail.code,
			Message: fail.message,
		},
		HunkIndex:          fail.hunkIndex,
		PatchLine:          fail.patchLine,
		Expected:           fail.expected,
		Candidates:         fail.candidates,
		MatchCount:         fail.matchCount,
		ExpectedMatchCount: fail.expectedMatchCount,
		MatchingLines:      fail.matchingLines,
	}
}
