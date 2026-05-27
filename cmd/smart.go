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
	describe           func(branch string, prRef string, llm string, thinking string, thinkingExplicit bool) error
	review             func(prRef string, llms string, thinking string, thinkingExplicit bool) error
}

func RunSmart(thinking string, thinkingExplicit bool) error {
	return runSmartWithOps(thinking, thinkingExplicit, smartOps{
		currentBranch:      git.CurrentBranch,
		currentPullRequest: git.GetCurrentBranchPullRequest,
		describe:           runDescribeWithThinking,
		review:             runReviewWithThinking,
	})
}

func runSmartWithOps(thinking string, thinkingExplicit bool, ops smartOps) error {
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
		return ops.describe("", "", smartDescribeLLM, thinking, thinkingExplicit)
	}

	prRef := strconv.Itoa(pr.Number)
	ui.Step("Found open pull request %s; reviewing with %s", ui.Emphasize("#"+prRef), ui.Emphasize(smartReviewLLMs))
	return ops.review(prRef, smartReviewLLMs, thinking, thinkingExplicit)
}
