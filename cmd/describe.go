package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
	"pr/pkg/ai"
	"pr/pkg/config"
	"pr/pkg/git"
	"pr/pkg/prompts"
	"pr/pkg/ui"
)

func extractPRDescription(input string) string {
	re := regexp.MustCompile(`(?s)<pr_description>\s*(.*?)\s*</pr_description>`)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1]
	}

	// Fallback if the AI fails to use tags: clean known watermarks and return raw.
	watermark := regexp.MustCompile(`(?m)^.*Generated with \[Claude Code\](?:\(https://claude\.com/claude-code\))?.*$`)
	clean := watermark.ReplaceAllString(input, "")
	return strings.TrimSpace(clean)
}

func parseMarkdownPR(description string) (string, string) {
	lines := strings.Split(description, "\n")
	title := "Auto-generated PR"
	bodyLines := []string{}
	foundTitle := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !foundTitle && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimPrefix(trimmed, "# ")
			foundTitle = true
		} else {
			if foundTitle || trimmed != "" {
				bodyLines = append(bodyLines, line)
			}
		}
	}
	return title, strings.TrimSpace(strings.Join(bodyLines, "\n"))
}

func DescribeCommand() *cli.Command {
	return &cli.Command{
		Name:     "describe",
		Usage:    ui.FormatReadUsage("Draft a PR title/body and create or update a GitHub PR"),
		Category: ui.FormatCategory("Code & PRs"), Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "br",
				Aliases:  []string{"b"},
				Usage:    "Optional. Branch to describe; uses the current branch if omitted",
				Required: false,
			},
			&cli.StringFlag{
				Name:    "pr",
				Usage:   "Optional. Existing GitHub PR number or URL to update instead of creating a new PR",
				Aliases: []string{"p"},
			},
			&cli.StringFlag{
				Name:    "llm",
				Usage:   "Optional. LLM to use (options: claude, gemini, codex, opencode)",
				Value:   "claude",
				Aliases: []string{"l"},
			},
			ThinkingFlag(),
		},
		Action: func(c *cli.Context) error {
			thinking, thinkingExplicit := ThinkingFromContext(c)
			return runDescribeWithThinking(c.String("br"), c.String("pr"), c.String("llm"), thinking, thinkingExplicit)
		},
	}
}

func runDescribe(branch string, prRef string, llm string) error {
	return runDescribeWithThinking(branch, prRef, llm, defaultThinkingLevel, false)
}

func runDescribeWithThinking(branch string, prRef string, llm string, thinking string, thinkingExplicit bool) error {
	guidelines, err := prompts.Load("describe")
	if err != nil {
		return fmt.Errorf("failed to load describe guidelines: %w", err)
	}
	if err := ai.ValidateThinking(llm, thinking, thinkingExplicit); err != nil {
		return err
	}

	if prRef != "" {
		if _, ok := git.ParsePullRequestRef(prRef); !ok {
			return fmt.Errorf("--pr must be a valid GitHub PR number or PR URL")
		}
	}

	var worktreeDir string
	var cleanup func()

	if branch == "" {
		currentBranch, err := git.CurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		if currentBranch == "" {
			return fmt.Errorf("not currently on any branch")
		}
		branch = currentBranch

		repoRoot, err := git.GetRepositoryRoot()
		if err != nil {
			return fmt.Errorf("failed to get repo root: %w", err)
		}

		worktreeDir = repoRoot
		cleanup = func() {
			os.Remove(filepath.Join(worktreeDir, ".pr-diff-context.md"))
		}

		ui.Step("Describing current branch %s", ui.Emphasize(branch))
	} else {
		ui.Step("Describing branch %s", ui.Emphasize(branch))
		ui.Step("Creating temporary worktree")

		tempDir, err := git.AddWorktree(branch)
		if err != nil {
			return err
		}
		ui.Note("Worktree: %s", tempDir)

		worktreeDir = tempDir
		cleanup = func() {
			ui.Note("Cleaning up worktree %s", tempDir)
			if err := git.RemoveWorktree(tempDir); err != nil {
				ui.Warn("Failed to remove worktree %s: %v", tempDir, err)
			}
		}
	}

	defer cleanup()

	diff, err := git.DiffAgainstBase(worktreeDir, "origin/main")
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	contextPath, err := writeBranchDiffContextFile(worktreeDir, "origin/main", diff)
	if err != nil {
		return fmt.Errorf("failed to write diff context: %w", err)
	}

	prompt := fmt.Sprintf(`Generate a Pull Request title and description for this branch. The project context and diff against origin/main are in the file %s in the current directory. Read that file before writing. If the project context points to domain-specific context files relevant to the diff, read those files too.

You MUST wrap your final output entirely within <pr_description> tags. Inside the tags, format the output as Markdown, with the PR title as an H1 heading (e.g. # My Title), followed by the description body. Do not output JSON.

Follow these PR description guidelines:
%s`, contextPath, guidelines)

	ui.Step("Running %s in read-only mode", ui.Emphasize(llm))

	response, err := ai.RunWithOptions(worktreeDir, prompt, llm, ai.ReadOnly, ai.RunOptions{ShowProgress: true, Verbose: true, Thinking: thinking, ThinkingExplicit: thinkingExplicit})
	if err != nil {
		return err
	}

	ui.Success("PR description prepared")

	description := extractPRDescription(response.FinalMessage)
	title, body := parseMarkdownPR(description)

	fmt.Printf("\n--- Prepared PR ---\nTitle: %s\n\n%s\n-------------------\n\n", title, body)

	confirmPrompt := "Do you want to create this Pull Request? [y/N]"
	if prRef != "" {
		confirmPrompt = "Do you want to update this Pull Request? [y/N]"
	}

	if !ui.Confirm(confirmPrompt) {
		if prRef != "" {
			ui.Warn("Operation aborted by user. PR was not updated.")
		} else {
			ui.Warn("Operation aborted by user. PR will not be created.")
		}
		return nil
	}

	if prRef != "" {
		ui.Step("Updating pull request %s", ui.Emphasize(prRef))
		if _, err := git.UpdatePR(worktreeDir, prRef, title, body); err != nil {
			return fmt.Errorf("failed to update PR: %w", err)
		}
		ui.Success("PR updated: %s", ui.Emphasize(prRef))
		return nil
	}

	var reviewers string
	if cfg, err := config.Load(); err == nil && cfg.GitHubReviewers != "" {
		reviewers = cfg.GitHubReviewers
	}

	ui.Step("Creating pull request")
	url, err := git.CreatePR(worktreeDir, title, body, reviewers)
	if err != nil {
		return fmt.Errorf("failed to create PR: %w", err)
	}
	if url != "" {
		ui.Success("PR created: %s", ui.Emphasize(url))
	} else {
		ui.Success("PR created successfully")
	}

	return nil
}
