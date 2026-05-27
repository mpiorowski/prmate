package cmd

import (
	"fmt"
	"strconv"

	"pr/pkg/git"
	"pr/pkg/ui"
)

const (
	smartDescribeLLM = "codex"
	smartReviewLLMs  = "codex,claude"
)

type smartOps struct {
	currentBranch      func() (string, error)
	currentPullRequest func() (git.PullRequest, bool, error)
	describe           func(branch string, prRef string, llm string) error
	review             func(prRef string, llms string) error
}

func RunSmart() error {
	return runSmartWithOps(smartOps{
		currentBranch:      git.CurrentBranch,
		currentPullRequest: git.GetCurrentBranchPullRequest,
		describe:           runDescribe,
		review:             runReview,
	})
}

func runSmartWithOps(ops smartOps) error {
	branch, err := ops.currentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if branch == "" {
		return fmt.Errorf("not currently on any branch")
	}

	ui.Step("Inspecting current branch %s", ui.Emphasize(branch))

	pr, found, err := ops.currentPullRequest()
	if err != nil {
		return fmt.Errorf("failed to inspect pull requests for current branch: %w", err)
	}

	if !found {
		ui.Step("No open pull request found; drafting one with %s", ui.Emphasize(smartDescribeLLM))
		return ops.describe("", "", smartDescribeLLM)
	}

	prRef := strconv.Itoa(pr.Number)
	ui.Step("Found open pull request %s; reviewing with %s", ui.Emphasize("#"+prRef), ui.Emphasize(smartReviewLLMs))
	return ops.review(prRef, smartReviewLLMs)
}
