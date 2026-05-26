package cmd

import (
	"fmt"

	"pr/pkg/git"
	"pr/pkg/ui"
)

type worktreeOps struct {
	addDetached          func(string) (string, error)
	fetchPullRequestHead func(int) (string, error)
	deleteRef            func(string) error
}

func addWorktreeFromPullRequestHeadWithOps(pr git.PullRequest, ops worktreeOps) (string, string, error) {
	if pr.HeadRefName != "" {
		tempDir, err := ops.addDetached("origin/" + pr.HeadRefName)
		if err == nil {
			return tempDir, "", nil
		}
	}

	fetchedRef, err := ops.fetchPullRequestHead(pr.Number)
	if err != nil {
		return "", "", fmt.Errorf("could not resolve PR #%d head: %w", pr.Number, err)
	}

	tempDir, err := ops.addDetached(fetchedRef)
	if err != nil {
		_ = ops.deleteRef(fetchedRef)
		return "", "", fmt.Errorf("could not create worktree from fetched ref: %w", err)
	}

	return tempDir, fetchedRef, nil
}

func addWorktreeFromPullRequestHead(pr git.PullRequest) (string, string, error) {
	return addWorktreeFromPullRequestHeadWithOps(pr, worktreeOps{
		addDetached:          git.AddWorktreeDetached,
		fetchPullRequestHead: git.FetchPullRequestHead,
		deleteRef:            git.DeleteRef,
	})
}

func addWorktreeFromBase(baseRef string) (string, error) {
	if baseRef != "" {
		tempDir, err := git.AddWorktreeDetached("origin/" + baseRef)
		if err == nil {
			return tempDir, nil
		}
	}
	return git.AddWorktreeDetached("origin/main")
}

func cleanupWorktree(tempDir string, fetchedRef string) {
	if tempDir != "" {
		ui.Note("Cleaning up worktree %s", tempDir)
		if err := git.RemoveWorktree(tempDir); err != nil {
			ui.Warn("Failed to remove worktree %s: %v", tempDir, err)
		}
	}
	if fetchedRef != "" {
		if err := git.DeleteRef(fetchedRef); err != nil {
			ui.Warn("Failed to delete fetched PR ref %s: %v", fetchedRef, err)
		}
	}
}
