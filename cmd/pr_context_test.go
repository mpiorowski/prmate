package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"pr/pkg/git"
)

func runContextTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}

	return strings.TrimSpace(string(out))
}

func TestWriteBranchDiffContextFileIncludesProjectContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CONTEXT.md"), []byte("# Project Rules\n\nRead me first.\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	name, err := writeBranchDiffContextFile(dir, "origin/main", "diff --git a/file.go b/file.go")
	if err != nil {
		t.Fatalf("writeBranchDiffContextFile returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(content)
	for _, want := range []string{
		"## Project Context",
		"<project_context>",
		"# Project Rules",
		"Read me first.",
		"## Branch Diff (against origin/main)",
		"diff --git a/file.go b/file.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context file missing %q:\n%s", want, got)
		}
	}
}

func TestWritePullRequestContextFileFallsBackToWorktreeProjectContext(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CONTEXT.md"), []byte("# Worktree Context\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	name, err := writePullRequestContextFile(dir, git.PullRequest{
		Number:      123,
		Title:       "Test PR",
		Body:        "Body",
		State:       "OPEN",
		BaseRefName: "main",
		HeadRefName: "feature/test",
	}, "diff --git a/file.go b/file.go")
	if err != nil {
		t.Fatalf("writePullRequestContextFile returned error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	got := string(content)
	for _, want := range []string{
		"# Pull Request Context",
		"## Project Context",
		"# Worktree Context",
		"## Diff",
		"diff --git a/file.go b/file.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("context file missing %q:\n%s", want, got)
		}
	}
}

func TestLoadPullRequestProjectContextPrefersBaseRef(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runContextTestGit(t, repo, "init", "-b", "main")
	runContextTestGit(t, repo, "config", "user.email", "test@example.com")
	runContextTestGit(t, repo, "config", "user.name", "Test User")

	contextPath := filepath.Join(repo, "CONTEXT.md")
	if err := os.WriteFile(contextPath, []byte("# Base Context\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	runContextTestGit(t, repo, "add", ".")
	runContextTestGit(t, repo, "commit", "-m", "base")
	runContextTestGit(t, repo, "branch", "origin/main")

	if err := os.WriteFile(contextPath, []byte("# PR Head Context\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	ctx, err := loadPullRequestProjectContext(repo, "main")
	if err != nil {
		t.Fatalf("loadPullRequestProjectContext returned error: %v", err)
	}
	if !ctx.found {
		t.Fatal("expected project context to be found")
	}
	if !strings.Contains(ctx.content, "# Base Context") {
		t.Fatalf("expected base context, got %q", ctx.content)
	}
	if strings.Contains(ctx.content, "# PR Head Context") {
		t.Fatalf("expected base context, got head context: %q", ctx.content)
	}
}
