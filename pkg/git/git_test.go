package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}

	return strings.TrimSpace(string(out))
}

func TestParsePullRequestRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "number", input: "123", want: "123", ok: true},
		{name: "github url", input: "https://github.com/example/repo/pull/456", want: "456", ok: true},
		{name: "github url with suffix", input: "https://github.com/example/repo/pull/789/files", want: "789", ok: true},
		{name: "branch name", input: "feature/test", want: "", ok: false},
		{name: "empty", input: "", want: "", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParsePullRequestRef(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePullRequestList(t *testing.T) {
	t.Parallel()

	pr, found, err := parsePullRequestList(`[{"number":123,"title":"Test PR","state":"OPEN","url":"https://github.com/example/repo/pull/123","baseRefName":"main","headRefName":"feature/test"}]`)
	if err != nil {
		t.Fatalf("parsePullRequestList returned error: %v", err)
	}
	if !found {
		t.Fatal("expected PR to be found")
	}
	if pr.Number != 123 || pr.Title != "Test PR" || pr.HeadRefName != "feature/test" {
		t.Fatalf("unexpected PR: %#v", pr)
	}
}

func TestParsePullRequest(t *testing.T) {
	t.Parallel()

	pr, found, err := parsePullRequest(`{"number":123,"title":"Test PR","state":"OPEN","url":"https://github.com/example/repo/pull/123","baseRefName":"main","headRefName":"feat/online-since-sidebar"}`)
	if err != nil {
		t.Fatalf("parsePullRequest returned error: %v", err)
	}
	if !found {
		t.Fatal("expected PR to be found")
	}
	if pr.Number != 123 || pr.Title != "Test PR" || pr.HeadRefName != "feat/online-since-sidebar" {
		t.Fatalf("unexpected PR: %#v", pr)
	}
}

func TestParsePullRequestRejectsEmptyPullRequest(t *testing.T) {
	t.Parallel()

	if _, _, err := parsePullRequest(`{}`); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsOpenPullRequest(t *testing.T) {
	t.Parallel()

	if !isOpenPullRequest(PullRequest{State: "OPEN"}) {
		t.Fatal("expected open PR to match")
	}
	if isOpenPullRequest(PullRequest{State: "CLOSED", Closed: true}) {
		t.Fatal("did not expect closed PR to match")
	}
	if isOpenPullRequest(PullRequest{State: "MERGED", Closed: true}) {
		t.Fatal("did not expect merged PR to match")
	}
}

func TestParsePullRequestListReturnsNotFoundForEmptyList(t *testing.T) {
	t.Parallel()

	pr, found, err := parsePullRequestList(`[]`)
	if err != nil {
		t.Fatalf("parsePullRequestList returned error: %v", err)
	}
	if found {
		t.Fatalf("expected no PR, got %#v", pr)
	}
}

func TestParsePullRequestListRejectsEmptyPullRequest(t *testing.T) {
	t.Parallel()

	if _, _, err := parsePullRequestList(`[{}]`); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsNoPullRequestForCurrentBranchError(t *testing.T) {
	t.Parallel()

	if !isNoPullRequestForCurrentBranchError(errors.New("exit status 1\nno pull requests found for branch \"main\"")) {
		t.Fatal("expected gh no-PR message to match")
	}

	if isNoPullRequestForCurrentBranchError(errors.New("error connecting to api.github.com")) {
		t.Fatal("did not expect unrelated gh error to match")
	}
}

func TestUpdatePRRejectsInvalidRef(t *testing.T) {
	t.Parallel()

	if _, err := UpdatePR(t.TempDir(), "feature/test", "title", "body"); err == nil {
		t.Fatal("expected error for invalid PR ref")
	}
}

func TestPushCurrentBranchPushesRenamedLocalBranch(t *testing.T) {
	t.Parallel()

	origin := filepath.Join(t.TempDir(), "origin.git")
	runGitCommand(t, filepath.Dir(origin), "init", "--bare", origin)

	repo := filepath.Join(t.TempDir(), "repo")
	runGitCommand(t, filepath.Dir(repo), "init", "-b", "main", repo)
	runGitCommand(t, repo, "config", "user.email", "test@example.com")
	runGitCommand(t, repo, "config", "user.name", "Test User")

	readmePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readmePath, []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGitCommand(t, repo, "add", ".")
	runGitCommand(t, repo, "commit", "-m", "initial")
	runGitCommand(t, repo, "remote", "add", "origin", origin)
	runGitCommand(t, repo, "push", "-u", "origin", "main")

	runGitCommand(t, repo, "checkout", "-b", "fix/old-name")
	if err := os.WriteFile(readmePath, []byte("updated\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGitCommand(t, repo, "add", ".")
	runGitCommand(t, repo, "commit", "-m", "update")
	runGitCommand(t, repo, "branch", "-m", "fix/readable-name")

	if err := pushCurrentBranch(repo); err != nil {
		t.Fatalf("pushCurrentBranch returned error: %v", err)
	}

	remoteHead := runGitCommand(t, repo, "ls-remote", "--heads", "origin", "fix/readable-name")
	if !strings.Contains(remoteHead, "refs/heads/fix/readable-name") {
		t.Fatalf("expected renamed branch to be pushed, got %q", remoteHead)
	}
}

func TestHasChanges(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	gitCmd := func(args ...string) error {
		t.Helper()
		cmdArgs := append([]string{"-C", repo}, args...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := gitCmd("init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := gitCmd("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := gitCmd("config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	filePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(filePath, []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := gitCmd("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := gitCmd("commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	hasChanges, err := HasChanges(repo)
	if err != nil {
		t.Fatalf("HasChanges returned error: %v", err)
	}
	if hasChanges {
		t.Fatal("expected clean repo to report no changes")
	}

	if err := os.WriteFile(filePath, []byte("updated\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	hasChanges, err = HasChanges(repo)
	if err != nil {
		t.Fatalf("HasChanges returned error: %v", err)
	}
	if !hasChanges {
		t.Fatal("expected dirty repo to report changes")
	}
}

func TestDiffAgainstBase(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	gitCmd := func(args ...string) error {
		t.Helper()
		cmdArgs := append([]string{"-C", repo}, args...)
		cmd := exec.Command("git", cmdArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if err := gitCmd("init", "-b", "main"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := gitCmd("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if err := gitCmd("config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	filePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(filePath, []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := gitCmd("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := gitCmd("commit", "-m", "initial"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
	if err := gitCmd("branch", "origin/main"); err != nil {
		t.Fatalf("git branch origin/main failed: %v", err)
	}
	if err := gitCmd("checkout", "-b", "feature/test"); err != nil {
		t.Fatalf("git checkout failed: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("updated\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := gitCmd("add", "."); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := gitCmd("commit", "-m", "update"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	diff, err := DiffAgainstBase(repo, "main")
	if err != nil {
		t.Fatalf("DiffAgainstBase returned error: %v", err)
	}
	if !strings.Contains(diff, "-initial") || !strings.Contains(diff, "+updated") {
		t.Fatalf("unexpected diff: %q", diff)
	}
}

func TestBranchExists(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	gitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	gitCmd("init", "-b", "main")
	gitCmd("config", "user.email", "test@example.com")
	gitCmd("config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	// Temporarily override git to run in our test repo by changing directory.
	// BranchExists runs git without -C, so we need to chdir.
	origDir, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	if !BranchExists("main") {
		t.Fatal("expected local branch 'main' to exist")
	}

	gitCmd("branch", "feature/test")
	if !BranchExists("feature/test") {
		t.Fatal("expected local branch 'feature/test' to exist")
	}

	if BranchExists("nonexistent-branch") {
		t.Fatal("expected nonexistent branch to return false")
	}
}

func TestDiffFull(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()

	gitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	gitCmd("init", "-b", "main")
	gitCmd("config", "user.email", "test@example.com")
	gitCmd("config", "user.name", "Test User")

	readmePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(readmePath, []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	gitCmd("add", ".")
	gitCmd("commit", "-m", "initial")

	// Clean repo should return empty diff
	diff, err := DiffFull(repo)
	if err != nil {
		t.Fatalf("DiffFull returned error: %v", err)
	}
	if diff != "" {
		t.Fatalf("expected empty diff for clean repo, got %q", diff)
	}

	// Modify a file — should appear in diff
	if err := os.WriteFile(readmePath, []byte("updated\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	diff, err = DiffFull(repo)
	if err != nil {
		t.Fatalf("DiffFull returned error: %v", err)
	}
	if !strings.Contains(diff, "-initial") || !strings.Contains(diff, "+updated") {
		t.Fatalf("expected patch content, got %q", diff)
	}

	// Add an untracked file — should also appear thanks to intent-to-add
	newFile := filepath.Join(repo, "new.txt")
	if err := os.WriteFile(newFile, []byte("brand new\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	diff, err = DiffFull(repo)
	if err != nil {
		t.Fatalf("DiffFull returned error: %v", err)
	}
	if !strings.Contains(diff, "new.txt") || !strings.Contains(diff, "+brand new") {
		t.Fatalf("expected untracked file in diff, got %q", diff)
	}
}

func TestFetchPullRequestHeadAndDeleteRef(t *testing.T) {
	origin := filepath.Join(t.TempDir(), "origin.git")
	runGitCommand(t, filepath.Dir(origin), "init", "--bare", origin)

	seed := filepath.Join(t.TempDir(), "seed")
	runGitCommand(t, filepath.Dir(seed), "init", "-b", "main", seed)
	runGitCommand(t, seed, "config", "user.email", "test@example.com")
	runGitCommand(t, seed, "config", "user.name", "Test User")

	readmePath := filepath.Join(seed, "README.md")
	if err := os.WriteFile(readmePath, []byte("initial\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGitCommand(t, seed, "add", ".")
	runGitCommand(t, seed, "commit", "-m", "initial")
	runGitCommand(t, seed, "remote", "add", "origin", origin)
	runGitCommand(t, seed, "push", "-u", "origin", "main")

	runGitCommand(t, seed, "checkout", "-b", "feature/pull-ref")
	if err := os.WriteFile(readmePath, []byte("feature change\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGitCommand(t, seed, "add", ".")
	runGitCommand(t, seed, "commit", "-m", "feature")
	featureCommit := runGitCommand(t, seed, "rev-parse", "HEAD")
	runGitCommand(t, seed, "push", "--force", "origin", "HEAD:refs/pull/123/head")

	consumer := filepath.Join(t.TempDir(), "consumer")
	runGitCommand(t, filepath.Dir(consumer), "clone", origin, consumer)

	ref, err := fetchPullRequestHead(consumer, 123)
	if err != nil {
		t.Fatalf("fetchPullRequestHead returned error: %v", err)
	}
	if ref != "refs/pr-tool/pr/123" {
		t.Fatalf("got ref %q", ref)
	}

	gotCommit := runGitCommand(t, consumer, "rev-parse", ref)
	if gotCommit != featureCommit {
		t.Fatalf("got commit %q, want %q", gotCommit, featureCommit)
	}

	if err := deleteRef(consumer, ref); err != nil {
		t.Fatalf("deleteRef returned error: %v", err)
	}

	cmd := exec.Command("git", "-C", consumer, "rev-parse", ref)
	if err := cmd.Run(); err == nil {
		t.Fatal("expected ref to be deleted")
	}
}

func TestReadFileAtRef(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	runGitCommand(t, repo, "init", "-b", "main")
	runGitCommand(t, repo, "config", "user.email", "test@example.com")
	runGitCommand(t, repo, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, "CONTEXT.md"), []byte("# Context\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGitCommand(t, repo, "add", ".")
	runGitCommand(t, repo, "commit", "-m", "context")

	content, ok, err := ReadFileAtRef(repo, "HEAD", "CONTEXT.md")
	if err != nil {
		t.Fatalf("ReadFileAtRef returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected file to be found")
	}
	if content != "# Context\n" {
		t.Fatalf("got %q", content)
	}

	_, ok, err = ReadFileAtRef(repo, "HEAD", "MISSING.md")
	if err != nil {
		t.Fatalf("ReadFileAtRef returned error for missing file: %v", err)
	}
	if ok {
		t.Fatal("did not expect missing file to be found")
	}
}
