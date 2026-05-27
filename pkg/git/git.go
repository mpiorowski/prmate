package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type PullRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	MergedAt    string `json:"mergedAt"`
	Closed      bool   `json:"closed"`
}

const pullRequestJSONFields = "number,title,body,state,url,baseRefName,headRefName,mergedAt,closed"

func pullRequestHeadRef(number int) string {
	return fmt.Sprintf("refs/pr-tool/pr/%d", number)
}

// BranchExists returns true if the given branch exists locally or at origin.
func BranchExists(branch string) bool {
	// Check local
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	if err := cmd.Run(); err == nil {
		return true
	}
	// Check remote
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
	return cmd.Run() == nil
}

// AddWorktree creates a git worktree for an existing branch.
// It fetches the branch from origin first to ensure it's up to date.
func AddWorktree(branch string) (string, error) {
	_ = FetchOrigin(branch) // best-effort; branch may only exist locally

	tempDir, err := os.MkdirTemp("", "pr-worktree-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", tempDir, branch)
	if _, err := runQuiet(cmd); err != nil {
		os.RemoveAll(tempDir) // cleanup on failure
		return "", fmt.Errorf("git worktree add failed: %w", err)
	}

	// Fast-forward local branch to remote tip so we don't push behind origin
	reset := exec.Command("git", "-C", tempDir, "reset", "--hard", "origin/"+branch)
	_ = reset.Run() // best-effort; origin/<branch> may not exist

	return tempDir, nil
}

// FetchOrigin fetches the latest state of a branch from origin.
func FetchOrigin(branch string) error {
	cmd := exec.Command("git", "fetch", "origin", branch)
	if _, err := runQuiet(cmd); err != nil {
		return fmt.Errorf("git fetch origin %s failed: %w", branch, err)
	}
	return nil
}

// AddWorktreeNewBranch creates a git worktree for a new branch based on a base branch.
// It fetches the base ref from origin first to ensure it's up to date.
func AddWorktreeNewBranch(branch, base string) (string, error) {
	if ref, ok := strings.CutPrefix(base, "origin/"); ok {
		_ = FetchOrigin(ref) // best-effort
	}

	tempDir, err := os.MkdirTemp("", "pr-worktree-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-b", branch, tempDir, base)
	if _, err := runQuiet(cmd); err != nil {
		os.RemoveAll(tempDir) // cleanup on failure
		return "", fmt.Errorf("git worktree add -b failed: %w", err)
	}

	return tempDir, nil
}

// AddWorktreeDetached creates a git worktree in a detached HEAD state.
// It fetches the ref from origin first to ensure it's up to date.
func AddWorktreeDetached(base string) (string, error) {
	if ref, ok := strings.CutPrefix(base, "origin/"); ok {
		_ = FetchOrigin(ref) // best-effort
	}

	tempDir, err := os.MkdirTemp("", "pr-worktree-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	cmd := exec.Command("git", "worktree", "add", "-d", tempDir, base)
	if _, err := runQuiet(cmd); err != nil {
		os.RemoveAll(tempDir) // cleanup on failure
		return "", fmt.Errorf("git worktree add -d failed: %w", err)
	}

	return tempDir, nil
}

// RemoveWorktree forcefully removes a git worktree.
func RemoveWorktree(path string) error {
	cmd := exec.Command("git", "worktree", "remove", path, "--force")
	if _, err := runQuiet(cmd); err != nil {
		return fmt.Errorf("git worktree remove failed: %w", err)
	}
	return nil
}

// CommitAndPush commits all changes and pushes to origin.
func CommitAndPush(path, message string) error {
	cmdAdd := exec.Command("git", "-C", path, "add", ".")
	if _, err := runQuiet(cmdAdd); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	cmdCommit := exec.Command("git", "-C", path, "commit", "-m", message)
	if _, err := runQuiet(cmdCommit); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	cmdPush := exec.Command("git", "-C", path, "push", "-u", "origin", "HEAD")
	if _, err := runQuiet(cmdPush); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

// HasChanges reports whether the worktree has any tracked or untracked modifications.
func HasChanges(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")

	out, err := runQuiet(cmd)
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}

	return strings.TrimSpace(out) != "", nil
}

// Status returns the short status of the worktree.
func Status(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "status", "-s")

	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("git status -s failed: %w", err)
	}

	return strings.TrimSpace(out), nil
}

// DiffStat returns the diff stat of the worktree against HEAD.
func DiffStat(path string) (string, error) {
	cmdAdd := exec.Command("git", "-C", path, "add", "-N", ".") // Intent-to-add so untracked files show up in diff
	_, _ = runQuiet(cmdAdd)

	cmd := exec.Command("git", "-C", path, "diff", "--stat", "HEAD")
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("git diff --stat failed: %w", err)
	}

	// Trim leading whitespace from each line (git pads filenames to right-align)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimLeft(line, " ")
	}
	return strings.Join(lines, "\n"), nil
}

// DiffFull returns the full patch diff of uncommitted changes (including untracked files) against HEAD.
func DiffFull(path string) (string, error) {
	cmdAdd := exec.Command("git", "-C", path, "add", "-N", ".") // Intent-to-add so untracked files show up
	_, _ = runQuiet(cmdAdd)

	cmd := exec.Command("git", "-C", path, "diff", "--patch", "--no-color", "HEAD")
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// CreatePR creates a GitHub pull request using the gh CLI.
func CreatePR(path, title, body, reviewers string) (string, error) {
	if err := pushCurrentBranch(path); err != nil {
		return "", err
	}

	args := []string{"pr", "create", "--title", title, "--body", body}
	if reviewers != "" {
		args = append(args, "--reviewer", reviewers)
	}
	cmd := exec.Command("gh", args...)
	cmd.Dir = path
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("gh pr create failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func pushCurrentBranch(path string) error {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD")
	branch, err := runQuiet(cmd)
	if err != nil {
		return fmt.Errorf("git rev-parse --abbrev-ref HEAD failed: %w", err)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		return fmt.Errorf("cannot create PR from detached HEAD")
	}

	cmd = exec.Command("git", "-C", path, "push", "-u", "origin", "HEAD")
	if _, err := runQuiet(cmd); err != nil {
		return fmt.Errorf("failed to push branch %s before creating PR: %w", branch, err)
	}

	return nil
}

// UpdatePR updates an existing GitHub pull request title/body using the gh CLI.
func UpdatePR(path, ref, title, body string) (string, error) {
	prRef, ok := ParsePullRequestRef(ref)
	if !ok {
		return "", fmt.Errorf("invalid pull request reference %q", ref)
	}

	args := []string{"pr", "edit", prRef, "--title", title, "--body", body}
	cmd := exec.Command("gh", args...)
	cmd.Dir = path
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("gh pr edit failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func ParsePullRequestRef(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}

	if regexp.MustCompile(`^\d+$`).MatchString(trimmed) {
		return trimmed, true
	}

	matches := regexp.MustCompile(`/pull/(\d+)`).FindStringSubmatch(trimmed)
	if len(matches) == 2 {
		return matches[1], true
	}

	return "", false
}

func GetPullRequest(ref string) (PullRequest, error) {
	prRef, ok := ParsePullRequestRef(ref)
	if !ok {
		return PullRequest{}, fmt.Errorf("invalid pull request reference %q", ref)
	}

	cmd := exec.Command(
		"gh", "pr", "view", prRef,
		"--json", pullRequestJSONFields,
	)
	out, err := runQuiet(cmd)
	if err != nil {
		return PullRequest{}, fmt.Errorf("gh pr view failed: %w", err)
	}

	var pr PullRequest
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("decode gh pr view output: %w", err)
	}
	if pr.Number == 0 {
		return PullRequest{}, fmt.Errorf("gh pr view returned empty pull request")
	}

	return pr, nil
}

func GetCurrentBranchPullRequest() (PullRequest, bool, error) {
	branch, err := CurrentBranch()
	if err != nil {
		return PullRequest{}, false, err
	}
	if branch == "" {
		return PullRequest{}, false, fmt.Errorf("not currently on any branch")
	}

	cmd := exec.Command("gh", "pr", "view", "--json", pullRequestJSONFields)
	out, err := runQuiet(cmd)
	if err != nil {
		if isNoPullRequestForCurrentBranchError(err) {
			return PullRequest{}, false, nil
		}
		return PullRequest{}, false, fmt.Errorf("gh pr view failed: %w", err)
	}

	pr, found, err := parsePullRequest(out)
	if err != nil || !found {
		return pr, found, err
	}
	if !isOpenPullRequest(pr) {
		return PullRequest{}, false, nil
	}

	return pr, true, nil
}

func isOpenPullRequest(pr PullRequest) bool {
	return !pr.Closed && strings.EqualFold(pr.State, "OPEN")
}

func GetPullRequestForBranch(branch string) (PullRequest, bool, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return PullRequest{}, false, fmt.Errorf("branch is required")
	}

	cmd := exec.Command(
		"gh", "pr", "list",
		"--head", branch,
		"--state", "open",
		"--limit", "1",
		"--json", pullRequestJSONFields,
	)
	out, err := runQuiet(cmd)
	if err != nil {
		return PullRequest{}, false, fmt.Errorf("gh pr list failed: %w", err)
	}

	return parsePullRequestList(out)
}

func parsePullRequestList(out string) (PullRequest, bool, error) {
	var prs []PullRequest
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return PullRequest{}, false, fmt.Errorf("decode gh pr list output: %w", err)
	}
	if len(prs) == 0 {
		return PullRequest{}, false, nil
	}
	if prs[0].Number == 0 {
		return PullRequest{}, false, fmt.Errorf("gh pr list returned empty pull request")
	}

	return prs[0], true, nil
}

func parsePullRequest(out string) (PullRequest, bool, error) {
	var pr PullRequest
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return PullRequest{}, false, fmt.Errorf("decode gh pr view output: %w", err)
	}
	if pr.Number == 0 {
		return PullRequest{}, false, fmt.Errorf("gh pr view returned empty pull request")
	}

	return pr, true, nil
}

func isNoPullRequestForCurrentBranchError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no pull requests found")
}

func GetPullRequestDiff(ref string) (string, error) {
	prRef, ok := ParsePullRequestRef(ref)
	if !ok {
		return "", fmt.Errorf("invalid pull request reference %q", ref)
	}

	cmd := exec.Command("gh", "pr", "diff", prRef, "--patch", "--color", "never")
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("gh pr diff failed: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func GetPRChecks(ref string) (string, error) {
	prRef, ok := ParsePullRequestRef(ref)
	if !ok {
		return "", fmt.Errorf("invalid pull request reference %q", ref)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := exec.Command("gh", "pr", "checks", prRef)
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	output := strings.TrimSpace(out.String() + "\n" + errOut.String())
	if err != nil {
		// gh pr checks exits with status 1 if any checks are failing.
		// We still want to return the output because it contains the failure details.
		if output != "" {
			return output, nil
		}
		return "", fmt.Errorf("gh pr checks failed: %w", err)
	}

	return output, nil
}

func FetchPullRequestHead(number int) (string, error) {
	return fetchPullRequestHead("", number)
}

func fetchPullRequestHead(repoDir string, number int) (string, error) {
	if number <= 0 {
		return "", fmt.Errorf("invalid pull request number %d", number)
	}

	localRef := pullRequestHeadRef(number)
	cmd := exec.Command("git", "fetch", "--force", "origin", fmt.Sprintf("pull/%d/head:%s", number, localRef))
	cmd.Dir = repoDir

	if _, err := runQuiet(cmd); err != nil {
		return "", fmt.Errorf("git fetch pull request head failed: %w", err)
	}

	return localRef, nil
}

func DeleteRef(ref string) error {
	return deleteRef("", ref)
}

func deleteRef(repoDir, ref string) error {
	if strings.TrimSpace(ref) == "" {
		return fmt.Errorf("ref is required")
	}

	cmd := exec.Command("git", "update-ref", "-d", ref)
	cmd.Dir = repoDir

	if _, err := runQuiet(cmd); err != nil {
		return fmt.Errorf("git update-ref -d failed: %w", err)
	}

	return nil
}

func DiffAgainstBase(path string, baseRef string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(baseRef) != "" {
		candidates = append(candidates, "origin/"+baseRef+"...HEAD", baseRef+"...HEAD")
	}
	candidates = append(candidates, "origin/main...HEAD", "main...HEAD")

	var lastErr error
	for _, candidate := range candidates {
		cmd := exec.Command("git", "-C", path, "diff", "--patch", "--no-color", candidate)
		out, err := runQuiet(cmd)
		if err == nil {
			return strings.TrimSpace(out), nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no base diff candidates available")
	}
	return "", fmt.Errorf("git diff against base failed: %w", lastErr)
}

func runQuiet(cmd *exec.Cmd) (string, error) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(out.String() + "\n" + errOut.String())
		if output != "" {
			return "", fmt.Errorf("%w\n%s", err, output)
		}
		return "", err
	}

	return out.String(), nil
}

func GetRepositoryRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// CurrentBranch returns the name of the current branch.
func CurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	out, err := runQuiet(cmd)
	if err != nil {
		return "", fmt.Errorf("git branch --show-current failed: %w", err)
	}
	return strings.TrimSpace(out), nil
}
