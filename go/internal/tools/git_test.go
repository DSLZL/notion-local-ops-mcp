package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	runGitCommand(t, root, "init", "-b", "main")
	runGitCommand(t, root, "config", "user.name", "Test User")
	runGitCommand(t, root, "config", "user.email", "test@example.com")
}

func runGitCommand(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func writeFileForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func TestGitStatusReportsStagedUnstagedAndUntracked(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	tracked := filepath.Join(root, "tracked.txt")
	writeFileForTest(t, tracked, "one\n")
	runGitCommand(t, root, "add", "tracked.txt")
	runGitCommand(t, root, "commit", "-m", "init")

	writeFileForTest(t, tracked, "two\n")
	staged := filepath.Join(root, "staged.txt")
	writeFileForTest(t, staged, "stage me\n")
	runGitCommand(t, root, "add", "staged.txt")
	writeFileForTest(t, filepath.Join(root, "new.txt"), "new\n")

	result := GitStatus(root)
	if !result.Success {
		t.Fatalf("GitStatus() success = false, err=%+v", result.Error)
	}
	if result.Clean {
		t.Fatal("clean = true, want false")
	}
	assertContains(t, result.Unstaged, "tracked.txt")
	assertContains(t, result.Staged, "staged.txt")
	assertContains(t, result.Untracked, "new.txt")
}

func TestGitDiffReturnsUnifiedDiff(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "app.py")
	writeFileForTest(t, target, "before\n")
	runGitCommand(t, root, "add", "app.py")
	runGitCommand(t, root, "commit", "-m", "init")
	writeFileForTest(t, target, "after\n")

	result := GitDiff(root, false, nil, 65536, 16384)
	if !result.Success {
		t.Fatalf("GitDiff() success = false, err=%+v", result.Error)
	}
	if len(result.Files) != 1 || result.Files[0] != "app.py" {
		t.Fatalf("files = %#v, want [app.py]", result.Files)
	}
	if !strings.Contains(result.Diff, "-before") || !strings.Contains(result.Diff, "+after") {
		t.Fatalf("diff = %q", result.Diff)
	}
}

func TestGitLogReturnsRecentCommits(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "note.txt")
	writeFileForTest(t, target, "v1\n")
	runGitCommand(t, root, "add", "note.txt")
	runGitCommand(t, root, "commit", "-m", "feat: add note")
	writeFileForTest(t, target, "v2\n")
	runGitCommand(t, root, "add", "note.txt")
	runGitCommand(t, root, "commit", "-m", "fix: update note")

	result := GitLog(root, 2)
	if !result.Success {
		t.Fatalf("GitLog() success = false, err=%+v", result.Error)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(result.Entries))
	}
	if result.Entries[0].Summary != "fix: update note" || result.Entries[1].Summary != "feat: add note" {
		t.Fatalf("summaries = %#v", result.Entries)
	}
}

func TestGitShowReturnsCommitMetadataAndDiff(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "note.txt")
	writeFileForTest(t, target, "v1\n")
	runGitCommand(t, root, "add", "note.txt")
	runGitCommand(t, root, "commit", "-m", "init")
	writeFileForTest(t, target, "v2\n")
	runGitCommand(t, root, "add", "note.txt")
	runGitCommand(t, root, "commit", "-m", "fix: update note")

	logResult := GitLog(root, 1)
	if !logResult.Success || len(logResult.Entries) == 0 {
		t.Fatalf("GitLog() failed: %+v", logResult.Error)
	}
	ref := logResult.Entries[0].Commit
	result := GitShow(root, ref, 65536, 16384)
	if !result.Success {
		t.Fatalf("GitShow() success = false, err=%+v", result.Error)
	}
	if result.Commit != ref {
		t.Fatalf("commit = %q, want %q", result.Commit, ref)
	}
	if result.Summary != "fix: update note" {
		t.Fatalf("summary = %q", result.Summary)
	}
	if len(result.Files) != 1 || result.Files[0] != "note.txt" {
		t.Fatalf("files = %#v", result.Files)
	}
	if !strings.Contains(result.Diff, "-v1") || !strings.Contains(result.Diff, "+v2") {
		t.Fatalf("diff = %q", result.Diff)
	}
}

func TestGitBlameReturnsPerLineCommitInfo(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "poem.txt")
	writeFileForTest(t, target, "roses\nviolets\n")
	runGitCommand(t, root, "add", "poem.txt")
	runGitCommand(t, root, "commit", "-m", "init")
	writeFileForTest(t, target, "roses\nviolets\nsugar\n")
	runGitCommand(t, root, "add", "poem.txt")
	runGitCommand(t, root, "commit", "-m", "add sugar")

	logResult := GitLog(root, 1)
	if !logResult.Success || len(logResult.Entries) == 0 {
		t.Fatalf("GitLog() failed: %+v", logResult.Error)
	}
	result := GitBlame(root, "poem.txt", "", nil, nil)
	if !result.Success {
		t.Fatalf("GitBlame() success = false, err=%+v", result.Error)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(result.Entries))
	}
	last := result.Entries[2]
	if last.Line != 3 || last.Content != "sugar" {
		t.Fatalf("last entry = %#v", last)
	}
	if last.Commit != logResult.Entries[0].Commit {
		t.Fatalf("last commit = %q, want %q", last.Commit, logResult.Entries[0].Commit)
	}
	if last.Summary != "add sugar" {
		t.Fatalf("summary = %q", last.Summary)
	}
}

func TestGitCommitCanStagePathsAndReturnCommitMetadata(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "feature.txt")
	writeFileForTest(t, target, "hello\n")

	result := GitCommit(root, "feat: add feature file", []string{"feature.txt"}, false, false, false, "", false, false)
	if !result.Success {
		t.Fatalf("GitCommit() success = false, err=%+v", result.Error)
	}
	if len(result.Commit) != 40 {
		t.Fatalf("commit len = %d, want 40", len(result.Commit))
	}
	if result.Summary != "feat: add feature file" {
		t.Fatalf("summary = %q", result.Summary)
	}
}

func TestGitCommitAmendUpdatesHead(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "note.txt")
	writeFileForTest(t, target, "v1\n")
	first := GitCommit(root, "feat: add note", []string{"note.txt"}, false, false, false, "", false, false)
	if !first.Success {
		t.Fatalf("first commit failed: %+v", first.Error)
	}
	writeFileForTest(t, target, "v2\n")

	amended := GitCommit(root, "feat: add note (amended)", []string{"note.txt"}, false, true, false, "", false, false)
	if !amended.Success {
		t.Fatalf("amended commit failed: %+v", amended.Error)
	}
	if !amended.Amended {
		t.Fatal("amended = false, want true")
	}
	if amended.Commit == first.Commit {
		t.Fatal("amended commit hash should differ from first commit hash")
	}
	logResult := GitLog(root, 5)
	if !logResult.Success {
		t.Fatalf("GitLog() failed: %+v", logResult.Error)
	}
	if len(logResult.Entries) != 1 || logResult.Entries[0].Summary != "feat: add note (amended)" {
		t.Fatalf("log entries = %#v", logResult.Entries)
	}
}

func TestGitCommitAllowEmptyCreatesCommitWithoutChanges(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "seed.txt")
	writeFileForTest(t, target, "s\n")
	first := GitCommit(root, "init", []string{"seed.txt"}, false, false, false, "", false, false)
	if !first.Success {
		t.Fatalf("first commit failed: %+v", first.Error)
	}

	result := GitCommit(root, "chore: empty", nil, false, false, true, "", false, false)
	if !result.Success {
		t.Fatalf("GitCommit() success = false, err=%+v", result.Error)
	}
	if !result.AllowEmpty {
		t.Fatal("allow_empty = false, want true")
	}
	if result.Summary != "chore: empty" {
		t.Fatalf("summary = %q", result.Summary)
	}
}

func TestGitCommitRejectsEmptyWhenAllowEmptyFalse(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "seed.txt")
	writeFileForTest(t, target, "s\n")
	first := GitCommit(root, "init", []string{"seed.txt"}, false, false, false, "", false, false)
	if !first.Success {
		t.Fatalf("first commit failed: %+v", first.Error)
	}

	result := GitCommit(root, "chore: empty", nil, false, false, false, "", false, false)
	if result.Success {
		t.Fatal("success = true, want false")
	}
	if result.Error == nil || result.Error.Code != "nothing_to_commit" {
		t.Fatalf("error = %+v, want nothing_to_commit", result.Error)
	}
}

func TestGitCommitDryRunReportsPlanWithoutCreatingCommit(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "note.txt")
	writeFileForTest(t, target, "v1\n")

	result := GitCommit(root, "feat: dry run", []string{"note.txt"}, false, false, false, "", false, true)
	if !result.Success {
		t.Fatalf("GitCommit() success = false, err=%+v", result.Error)
	}
	if !result.DryRun {
		t.Fatal("dry_run = false, want true")
	}
	if result.Summary != "feat: dry run" {
		t.Fatalf("summary = %q", result.Summary)
	}
	assertContains(t, result.Files, "note.txt")
	if result.Commit != "" {
		t.Fatalf("commit = %q, want empty in dry run", result.Commit)
	}
}

func TestGitCommitRespectsCustomAuthor(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	target := filepath.Join(root, "file.txt")
	writeFileForTest(t, target, "x\n")

	result := GitCommit(root, "feat: thing", []string{"file.txt"}, false, false, false, "Alice <alice@example.com>", false, false)
	if !result.Success {
		t.Fatalf("GitCommit() success = false, err=%+v", result.Error)
	}
	logResult := GitLog(root, 1)
	if !logResult.Success {
		t.Fatalf("GitLog() failed: %+v", logResult.Error)
	}
	if len(logResult.Entries) != 1 || logResult.Entries[0].Author != "Alice" {
		t.Fatalf("log entries = %#v", logResult.Entries)
	}
}

func assertContains(t *testing.T, values []string, target string) {
	t.Helper()
	for _, value := range values {
		if value == target {
			return
		}
	}
	t.Fatalf("%q not found in %#v", target, values)
}
